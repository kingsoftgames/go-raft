package store

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/common"
	"github.com/golang/protobuf/proto"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 4 * time.Second
	raftLogCacheSize    = 512
)

type ValueType interface{}
type StoreType map[string]ValueType

type RaftFuture struct {
	responsed bool
	err       chan error
	e         error
	rsp       interface{}
}

func (th *RaftFuture) response(err error) {
	th.err <- err
}
func (th *RaftFuture) Error() error {
	if th.responsed {
		return th.e
	}
	timer := time.After(5 * time.Second)
	select {
	case <-timer:
		th.e = errors.New("timeout")
		break
	case th.e = <-th.err:
		break
	}
	th.responsed = true
	return th.e
}
func (th *RaftFuture) Response() interface{} {
	if !th.responsed {
		return nil
	}
	return th.rsp
}
func (th *RaftFuture) Index() uint64 {
	return 0
}
func (th *StoreType) copyFrom(v StoreType) {
	for k, v := range v {
		(*th)[k] = v
	}
}

type raftServers []raft.Server

func (th *raftServers) put(id, addr string) {
	for i, s := range *th {
		if s.ID == raft.ServerID(id) {
			(*th)[i].Address = raft.ServerAddress(addr)
			return
		}
	}
	*th = append(*th, raft.Server{
		ID:      raft.ServerID(id),
		Address: raft.ServerAddress(addr),
	})
}

type RaftStore struct {
	config *RaftConfigure
	l      sync.RWMutex
	m      StoreType

	servers     raftServers
	raft        *raft.Raft
	raftStore   *raftboltdb.BoltStore
	logStore    *LogStoreCache
	OnStateChg  common.SafeEvent
	OnLeaderChg common.SafeEvent
	OnLeader    common.SafeEvent
	OnPeerAdd   common.SafeEvent

	transport *raft.NetworkTransport

	peers sync.Map

	runChan common.RunChanType

	exitChan chan struct{}

	goFunc common.GoFunc

	leader   bool
	exitWait sync.WaitGroup

	statTicker *common.Ticker
	stopped    bool

	runLogic common.LogicChan

	expireKey    map[string]int64
	expireTimer  *common.Timer
	OnKeysExpire common.SafeEvent
	keyChan      chan *expireKey
}

func New(config *RaftConfigure, runChan common.RunChanType, goFunc common.GoFunc) *RaftStore {
	return &RaftStore{
		config:   config,
		m:        make(StoreType),
		runChan:  runChan,
		servers:  raftServers{},
		goFunc:   goFunc,
		exitChan: make(chan struct{}, 1),
		stopped:  false,
	}
}
func (th *RaftStore) GetMSize() int {
	th.l.RLock()
	defer th.l.RUnlock()
	return len(th.m)
}
func (th *RaftStore) getNodeName() string {
	return th.config.NodeId
}

type expireKey struct {
	k string
	r bool
}

func (th *RaftStore) runExpire() {
	if th.config.KeyExpireS == 0 {
		return
	}
	th.expireKey = map[string]int64{}
	th.keyChan = make(chan *expireKey)
	timerChan := make(chan struct{})
	ticker := common.NewTicker(time.Second, func() {
		timerChan <- struct{}{}
	})
	now := time.Now().Unix()
	th.goFunc.Go(func() {
		defer ticker.Stop()
		var cnt uint64
		for {
			select {
			case key := <-th.keyChan:
				if key == nil {
					logrus.Infof("[%s]runExpire finished", th.getNodeName())
					return
				}
				if key.r {
					delete(th.expireKey, key.k)
				} else {
					th.expireKey[key.k] = now
				}
			case <-timerChan:
				now = time.Now().Unix()
				cnt++
				if cnt%uint64(th.config.KeyExpireS) == 0 {
					if th.IsLeader() {
						expireKeys := make([]string, 0)
						for k, t := range th.expireKey {
							if now-t > int64(th.config.KeyExpireS) {
								expireKeys = append(expireKeys, k)
							}
						}
						if len(expireKeys) > 0 {
							th.goFunc.GoN(func(p ...interface{}) {
								th.OnKeysExpire.Emit(p[0])
							}, expireKeys)
						}
					}
				}
			}
		}
	})
}
func (th *RaftStore) closeExpire() {
	if th.keyChan != nil {
		close(th.keyChan)
		th.keyChan = nil
	}
}
func (th *RaftStore) liveKey(key string, remove bool) {
	if th.config.KeyExpireS == 0 {
		return
	}
	th.goFunc.Go(func() {
		th.keyChan <- &expireKey{
			k: key,
			r: remove,
		}
	})
}
func (th *RaftStore) runApply() {
	th.runLogic.Init(th.config.RunChanNum)
	th.goFunc.Go(func() {
		th.runLogic.Start()
	})
}
func (th *RaftStore) applyRun(key string, cmd *inner.ApplyCmd) raft.ApplyFuture {
	th.liveKey(key, cmd.Cmd == inner.ApplyCmd_DEL)
	if th.config.RaftApplyHash {
		hash := common.GetStringSum(key)
		rsp := &RaftFuture{
			err: make(chan error),
		}
		if !th.runLogic.Stopped() {
			th.runLogic.Handle(hash, 5*time.Second, func(err error) {
				if err == nil {
					f := th.apply(cmd)
					if err := f.Error(); err != nil {
						rsp.response(err)
					} else {
						rsp.rsp = f.Response()
						rsp.response(nil)
					}
				} else {
					rsp.response(err)
				}
			})
		} else {
			go rsp.response(fmt.Errorf("node are stopping"))
		}
		return rsp
	} else {
		return th.apply(cmd)
	}
}
func (th *RaftStore) apply(cmd *inner.ApplyCmd) raft.ApplyFuture {
	data, err := proto.Marshal(cmd)
	if err != nil {
		return nil
	}
	return th.raft.Apply(data, raftTimeout)

}
func (th *RaftStore) GetRaft() *raft.Raft {
	return th.raft
}
func (th *RaftStore) Foreach(fn func(string, ValueType)) {
	th.l.RLock()
	defer th.l.RUnlock()
	for k, v := range th.m {
		fn(k, v)
	}
}
func (th *RaftStore) Get(key string) (ValueType, error) {
	th.l.RLock()
	defer th.l.RUnlock()
	th.liveKey(key, false)
	return th.m[key], nil
}
func (th *RaftStore) Set(key string, value ValueType) raft.ApplyFuture {
	v, e := common.Encode(value)
	if e != nil {
		return nil
	}
	cmd := &inner.ApplyCmd{
		Cmd: inner.ApplyCmd_SET,
		Obj: &inner.ApplyCmd_Set{Set: &inner.ApplyCmd_OpSet{Key: key, Value: v}},
	}
	return th.applyRun(key, cmd)
}

func (th *RaftStore) Delete(key string) raft.ApplyFuture {
	cmd := &inner.ApplyCmd{
		Cmd: inner.ApplyCmd_DEL,
		Obj: &inner.ApplyCmd_Del{Del: &inner.ApplyCmd_OpDel{Key: key}},
	}
	return th.applyRun(key, cmd)
}

//get real-time data
func (th *RaftStore) GetAsync(key string, fn func(err error, valueType ValueType)) {
	th.goFunc.Go(func() {
		cmd := &inner.ApplyCmd{
			Cmd: inner.ApplyCmd_GET,
			Obj: &inner.ApplyCmd_Get{Get: &inner.ApplyCmd_OpGet{Key: key}},
		}
		f := th.applyRun(key, cmd)
		err := f.Error()
		if th.runChan != nil {
			th.runChan <- func() {
				fn(err, f.Response())
			}
		} else {
			fn(err, f.Response())
		}
	})
}
func (th *RaftStore) SetAsync(key string, valueType ValueType, fn func(err error, rsp interface{})) {
	th.goFunc.Go(func() {
		f := th.Set(key, valueType)
		err := f.Error()
		if th.runChan != nil {
			th.runChan <- func() {
				fn(err, f.Response())
			}
		} else {
			fn(err, f.Response())
		}
	})
}
func (th *RaftStore) DeleteAsync(key string, fn func(err error, rsp interface{})) {
	th.goFunc.Go(func() {
		f := th.Delete(key)
		err := f.Error()
		if th.runChan != nil {
			th.runChan <- func() {
				fn(err, f.Response())
			}
		} else {
			fn(err, f.Response())
		}
	})
}
func (th *RaftStore) Join(nodeId string, addr string) error {
	logrus.Infof("[%s]Join %s,%s", th.getNodeName(), nodeId, addr)
	configFuture := th.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}
	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeId) || srv.Address == raft.ServerAddress(addr) {
			if srv.ID == raft.ServerID(nodeId) && srv.Address == raft.ServerAddress(addr) {
				logrus.Infof("already join ,ignore")
				return nil
			}
			f := th.raft.RemoveServer(srv.ID, 0, 0)
			if err := f.Error(); err != nil {
				return err
			}
		}
	}
	f := th.raft.AddVoter(raft.ServerID(nodeId), raft.ServerAddress(addr), 0, 0)
	if err := f.Error(); err != nil {
		return err
	}
	logrus.Infof("join succeed")
	return nil
}
func (th *RaftStore) Apply(log *raft.Log) interface{} {
	var cmd inner.ApplyCmd
	if err := proto.Unmarshal(log.Data, &cmd); err != nil {
		logrus.Infof("Apply error %s", err.Error())
		return nil
	}
	//logrus.Debugf("[Store][%s]Apply %v,%d,%d", th.getNodeName(), cmd.Cmd, log.Term, log.Index)
	switch cmd.Cmd {
	case inner.ApplyCmd_GET:
		return th.applyGet(cmd.GetGet())
	case inner.ApplyCmd_SET:
		common.Debugf("[Store][%s]Apply Set %s", th.getNodeName(), cmd.GetSet().Key)
		return th.applySet(cmd.GetSet())
	case inner.ApplyCmd_DEL:
		return th.applyDel(cmd.GetDel())
	}
	return nil
}
func (th *RaftStore) applyGet(obj *inner.ApplyCmd_OpGet) interface{} {
	th.l.RLock()
	defer th.l.RUnlock()
	return th.m[obj.Key]
}
func (th *RaftStore) applySet(obj *inner.ApplyCmd_OpSet) interface{} {
	th.l.Lock()
	defer th.l.Unlock()
	th.m[obj.Key] = obj.Value
	return obj.Value
}
func (th *RaftStore) applyDel(obj *inner.ApplyCmd_OpDel) interface{} {
	th.l.Lock()
	defer th.l.Unlock()
	delete(th.m, obj.Key)
	return nil
}
func (th *RaftStore) Snapshot() (raft.FSMSnapshot, error) {
	logrus.Infof("[%s]Snapshot", th.getNodeName())
	th.l.RLock()
	defer th.l.RUnlock()
	sf := &storeFsmSnapshot{
		s: make(StoreType),
	}
	sf.s.copyFrom(th.m)
	return sf, nil
}
func (th *RaftStore) Restore(rc io.ReadCloser) error {
	logrus.Infof("[%s]Restore", th.getNodeName())
	o := make(StoreType)
	if err := common.DecodeFromReader(rc, &o); err != nil {
		return err
	}
	th.m = o
	return nil
}
func (th *RaftStore) initRaftConfig() *raft.Config {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(th.getNodeName())
	if th.config.SnapshotIntervalS > 0 {
		config.SnapshotInterval = time.Duration(th.config.SnapshotIntervalS) * time.Second
	}
	config.ShutdownOnRemove = false
	if th.config.MaxAppendEntries > 0 {
		config.MaxAppendEntries = th.config.MaxAppendEntries
	}
	if th.config.ElectionTimeoutMS != 0 {
		config.ElectionTimeout = time.Duration(th.config.ElectionTimeoutMS) * time.Millisecond
	}
	if th.config.HeartbeatTimeoutMS != 0 {
		config.HeartbeatTimeout = time.Duration(th.config.HeartbeatTimeoutMS) * time.Millisecond
	}
	if th.config.LeaderLeaseTimeoutMS != 0 {
		config.LeaderLeaseTimeout = time.Duration(th.config.LeaderLeaseTimeoutMS) * time.Millisecond
	}
	if th.config.PerformanceMultiplier > 0 {
		config.ElectionTimeout *= time.Duration(th.config.PerformanceMultiplier)
		config.HeartbeatTimeout *= time.Duration(th.config.PerformanceMultiplier)
		config.LeaderLeaseTimeout *= time.Duration(th.config.PerformanceMultiplier)
	}
	return config
}
func (th *RaftStore) Open(logLevel string, logOutput io.Writer) error {
	config := th.initRaftConfig()
	config.LogOutput = logOutput
	config.LogLevel = logLevel
	addr, err := net.ResolveTCPAddr("tcp", th.config.Addr)
	if err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(th.config.Addr, addr, 3, raftTimeout, logOutput)
	if err != nil {
		return err
	}
	th.transport = transport
	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshot raft.SnapshotStore
	if th.config.StoreInMem {
		if th.config.LogCacheCapacity > 0 {
			//log?????????????????????
			snapshot, err = raft.NewFileSnapshotStore(th.config.StoreDir, retainSnapshotCount, logOutput)
			if err != nil {
				return fmt.Errorf("new snapshot : %s", err.Error())
			}
			boltDB, err := raftboltdb.NewBoltStore(filepath.Join(th.config.StoreDir, "log.db"))
			if err != nil {
				return fmt.Errorf("new bolt store: %s", err.Error())
			}
			th.raftStore = boltDB
			if th.logStore, err = NewLogStoreCache(th.config.LogCacheCapacity, boltDB); err != nil {
				return fmt.Errorf("NewLogStoreCache err,%s", err)
			}
			logStore = th.logStore
			stableStore = boltDB
		} else {
			logStore = raft.NewInmemStore()
			stableStore = raft.NewInmemStore()
			snapshot = raft.NewInmemSnapshotStore()
		}
	} else {
		snapshot, err = raft.NewFileSnapshotStore(th.config.StoreDir, retainSnapshotCount, logOutput)
		if err != nil {
			return err
		}
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(th.config.StoreDir, "log.db"))
		if err != nil {
			return fmt.Errorf("new bolt store: %s", err)
		}
		th.raftStore = boltDB

		cacheStore, err := raft.NewLogCache(raftLogCacheSize, boltDB)
		if err != nil {
			return err
		}
		logStore = cacheStore
		stableStore = boltDB
	}
	//join self
	th.servers.put(th.getNodeName(), th.config.Addr)
	if th.config.Bootstrap {
		hasState, err := raft.HasExistingState(logStore, stableStore, snapshot)
		if err != nil {
			return err
		}
		if !hasState {
			logrus.Infof("Start as BootstrapCluster")
			if err := raft.BootstrapCluster(config, logStore, stableStore, snapshot, transport, raft.Configuration{
				Servers: th.servers,
			}); err != nil {
				return err
			}
		}
	}
	ra, err := raft.NewRaft(config, th, logStore, stableStore, snapshot, transport)
	if err != nil {
		return err
	}
	th.raft = ra
	th.runObserver()
	th.runApply()
	th.runExpire()
	return nil
}
func (th *RaftStore) clearStoreCache() {
	if err := os.RemoveAll(th.config.StoreDir); err != nil {
		logrus.Errorf("[%s]clearStoreCache,err,%s", th.getNodeName(), err.Error())
	} else {
		logrus.Infof("[%s]clearStoreCache finished", th.getNodeName())
	}
}
func (th *RaftStore) AddServer(id, addr string) {
	th.servers.put(id, addr)
}
func (th *RaftStore) BootStrap() error {
	logrus.Infof("[%s]RaftStore,BootStrap", th.getNodeName())
	return th.raft.BootstrapCluster(raft.Configuration{
		Servers: th.servers,
	}).Error()
}
func (th *RaftStore) runObserver() {
	obchan := make(chan raft.Observation, 1024)
	obsrv := raft.NewObserver(obchan, true, func(o *raft.Observation) bool {
		return true
	})
	th.raft.RegisterObserver(obsrv)
	th.goFunc.Go(func() {
		defer func() {
			close(obchan)
			th.exitWait.Done()
		}()
		//observer,?????????????????????????????????????????????????????????block???true??????channel?????????????????????????????????channel??????
		for {
			select {
			case obs := <-obchan:
				switch obs.Data.(type) {
				case *raft.RequestVoteRequest:
					ob := obs.Data.(*raft.RequestVoteRequest)
					logrus.Infof("[Observer][%s][%d]RequestVoteRequest,%v", th.raft.LastIndex(), th.getNodeName(), *ob)
				case raft.RaftState:
					ob := obs.Data.(raft.RaftState)
					th.eventEmit(&th.OnStateChg, ob)
					th.setLeader(ob == raft.Leader)
				case raft.PeerObservation:
					ob := obs.Data.(raft.PeerObservation)
					logrus.Infof("[Observer][%s][%d]PeerObservation,%v", th.getNodeName(), th.raft.LastIndex(), ob)
					if ob.Removed {
						th.peers.Delete(ob.Peer.ID)
					} else {
						th.peers.Store(ob.Peer.ID, ob.Peer.Address)
						th.eventEmit(&th.OnPeerAdd, string(ob.Peer.ID))
					}
				case raft.LeaderObservation:
					ob := obs.Data.(raft.LeaderObservation)
					if !th.IsLeader() { //??????leader???addr
						th.eventEmit(&th.OnLeaderChg, string(ob.Leader))
					} else {
						th.setLeader(string(ob.Leader) == th.config.Addr)
					}
					logrus.Infof("[Observer][%s][%d]LeaderObservation,%s", th.getNodeName(), th.raft.LastIndex(), ob.Leader)
				}
			case leader := <-th.raft.LeaderCh():
				logrus.Infof("[Observer][%s][%d]Leader,%v", th.getNodeName(), th.raft.LastIndex(), leader)
				th.setLeader(leader)
			case <-th.exitChan:
				logrus.Infof("[Observer][%s][%d]Exit", th.getNodeName(), th.raft.LastIndex())
				return
			}
		}
	})

	th.statTicker = common.NewTicker(time.Duration(th.config.PrintStateIntervalS)*time.Second, func() {
		logrus.Debugf("[%s][%v]%v", th.getNodeName(), th.raft.State(), th.raft.Stats())
	})
}
func (th *RaftStore) eventEmit(ev *common.SafeEvent, arg interface{}) {
	if th.runChan != nil {
		th.runChan <- func() {
			ev.EmitSafe(arg)
		}
	} else {
		ev.EmitSafe(arg)
	}
}

//??????????????????
func (th *RaftStore) setLeader(leader bool) {
	logrus.Infof("[%s]setLeader,%v,%v", th.getNodeName(), th.leader, leader)
	if th.leader != leader {
		th.leader = leader
		th.eventEmit(&th.OnLeader, leader)
	}
}
func (th *RaftStore) IsLeader() bool {
	return th.raft.State() == raft.Leader
}
func (th *RaftStore) IsFollower() bool {
	return th.raft.State() == raft.Follower
}
func (th *RaftStore) release() {
	th.raft = nil
}

//?????????
func (th *RaftStore) Shutdown() {
	if th.stopped {
		return
	}
	th.stopped = true
	if th.raft != nil {
		th.closeExpire()
		th.runLogic.Stop()
		th.transport.Close()
		f := th.raft.Shutdown()
		if e := f.Error(); e != nil {
			logrus.Warn("shutdown raft err,%s", e.Error())
		}
		if th.exitChan != nil {
			th.exitWait.Add(1)
			th.exitChan <- struct{}{}
			//th.exitWait.Wait()
		}
		if th.raftStore != nil {
			th.raftStore.Close()
		}
		if th.logStore != nil {
			th.logStore.Close()
		}
		if th.statTicker != nil {
			th.statTicker.Stop()
			th.statTicker = nil
		}
		th.clearStoreCache()
	}
}
func (th *RaftStore) GetNodes() []raft.Server {
	if f := th.raft.GetConfiguration(); f.Error() == nil {
		return f.Configuration().Servers
	}
	return nil
}

func (th *RaftStore) ReloadNode() {
	nodes := th.GetNodes()
	logrus.Debugf("[%s]ReloadNode,%v", th.getNodeName(), nodes)
	for _, node := range nodes {
		th.peers.Store(node.ID, node.Address)
	}
}

type storeFsmSnapshot struct {
	s StoreType
}

func (f *storeFsmSnapshot) Persist(sink raft.SnapshotSink) error {
	fmt.Printf("Persist")
	err := func() error {
		// Encode data.
		b, err := common.Encode(f.s)
		if err != nil {
			return err
		}

		// Write data to sink.
		if _, err := sink.Write(b); err != nil {
			return err
		}

		// Close the sink.
		return sink.Close()
	}()

	if err != nil {
		sink.Cancel()
	}

	return err
}

func (f *storeFsmSnapshot) Release() {
	fmt.Printf("Release")
}
