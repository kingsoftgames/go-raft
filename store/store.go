package store

import (
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

func (th *StoreType) copyFrom(v StoreType) {
	for k, v := range v {
		(*th)[k] = v
	}
}

func GetApiKey(raftAddr string) string {
	return "api-" + raftAddr
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
	config *common.Configure
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
}

func New(config *common.Configure, runChan common.RunChanType, goFunc common.GoFunc) *RaftStore {
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
	return th.apply(cmd)
}
func (th *RaftStore) Delete(key string) raft.ApplyFuture {
	cmd := &inner.ApplyCmd{
		Cmd: inner.ApplyCmd_DEL,
		Obj: &inner.ApplyCmd_Del{Del: &inner.ApplyCmd_OpDel{Key: key}},
	}
	return th.apply(cmd)
}

//get real-time data
func (th *RaftStore) GetAsync(key string, fn func(err error, valueType ValueType)) {
	th.goFunc.Go(func() {
		cmd := &inner.ApplyCmd{
			Cmd: inner.ApplyCmd_GET,
			Obj: &inner.ApplyCmd_Get{Get: &inner.ApplyCmd_OpGet{Key: key}},
		}
		f := th.apply(cmd)
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
	logrus.Infof("[%s]Join %s,%s", th.config.NodeId, nodeId, addr)
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
	//logrus.Debugf("[Store][%s]Apply %d", th.config.NodeId, cmd.Cmd)
	switch cmd.Cmd {
	case inner.ApplyCmd_GET:
		return th.applyGet(cmd.GetGet())
	case inner.ApplyCmd_SET:
		common.Debugf("[Store][%s]Apply Set %s", th.config.NodeId, cmd.GetSet().Key)
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
	logrus.Infof("[%s]Snapshot", th.config.NodeId)
	th.l.RLock()
	defer th.l.RUnlock()
	sf := &storeFsmSnapshot{
		s: make(StoreType),
	}
	sf.s.copyFrom(th.m)
	return sf, nil
}
func (th *RaftStore) Restore(rc io.ReadCloser) error {
	logrus.Infof("[%s]Restore", th.config.NodeId)
	o := make(StoreType)
	if err := common.DecodeFromReader(rc, &o); err != nil {
		return err
	}
	th.m = o
	return nil
}

func (th *RaftStore) Open(logLevel string, logOutput io.Writer) error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(th.config.NodeId)
	config.LogOutput = logOutput
	config.LogLevel = logLevel
	config.SnapshotInterval = 100 * time.Second
	config.ShutdownOnRemove = false
	//config.SnapshotInterval = 10 * time.Millisecond
	addr, err := net.ResolveTCPAddr("tcp", th.config.RaftAddr)
	if err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(th.config.RaftAddr, addr, 3, raftTimeout, logOutput)
	if err != nil {
		return err
	}
	th.transport = transport
	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshot raft.SnapshotStore
	if th.config.StoreInMem {
		if th.config.LogCacheCapacity > 0 {
			if th.logStore, err = NewLogStoreCache(th.config.LogCacheCapacity, th.config.StoreDir); err != nil {
				return fmt.Errorf("NewLogStoreCache err,%s", err)
			}
			logStore = th.logStore
		} else {
			logStore = raft.NewInmemStore()
		}
		stableStore = raft.NewInmemStore()
		//snapshot = raft.NewInmemSnapshotStore()
		snapshot, err = raft.NewFileSnapshotStore(th.config.StoreDir, retainSnapshotCount, logOutput)
	} else {
		snapshot, err = raft.NewFileSnapshotStore(th.config.StoreDir, retainSnapshotCount, logOutput)
		if err != nil {
			return fmt.Errorf("new snapshot store: %s", err)
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
	th.servers.put(th.config.NodeId, th.config.RaftAddr)
	if len(th.config.JoinAddr) == 0 && th.config.Bootstrap {
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
	return nil
}
func (th *RaftStore) clearStoreCache() {
	if err := os.RemoveAll(th.config.StoreDir); err != nil {
		logrus.Errorf("[%s]clearStoreCache,err,%s", th.config.NodeId, err.Error())
	} else {
		logrus.Infof("[%s]clearStoreCache finished", th.config.NodeId)
	}
}
func (th *RaftStore) AddServer(id, addr string) {
	th.servers.put(id, addr)
}
func (th *RaftStore) BootStrap() error {
	logrus.Infof("[%s]RaftStore,BootStrap", th.config.NodeId)
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
		//observer,管道大小弄大点，不然投票容易超时，因为block为true时，channel处理不过来会造成投票的channel阻塞
		for {
			select {
			case obs := <-obchan:
				switch obs.Data.(type) {
				case *raft.RequestVoteRequest:
					ob := obs.Data.(*raft.RequestVoteRequest)
					logrus.Infof("[Observer][%s][%d]RequestVoteRequest,%v", th.raft.LastIndex(), th.config.NodeId, *ob)
				case raft.RaftState:
					ob := obs.Data.(raft.RaftState)
					th.eventEmit(&th.OnStateChg, ob)
					th.setLeader(ob == raft.Leader)
				case raft.PeerObservation:
					ob := obs.Data.(raft.PeerObservation)
					logrus.Infof("[Observer][%s][%d]PeerObservation,%v", th.config.NodeId, th.raft.LastIndex(), ob)
					if ob.Removed {
						th.peers.Delete(ob.Peer.ID)
					} else {
						th.peers.Store(ob.Peer.ID, ob.Peer.Address)
						th.eventEmit(&th.OnPeerAdd, string(ob.Peer.ID))
					}
				case raft.LeaderObservation:
					ob := obs.Data.(raft.LeaderObservation)
					if !th.IsLeader() { //通知leader的addr
						th.eventEmit(&th.OnLeaderChg, string(ob.Leader))
					} else {
						th.setLeader(string(ob.Leader) == th.config.RaftAddr)
					}
					logrus.Infof("[Observer][%s][%d]LeaderObservation,%s", th.config.NodeId, th.raft.LastIndex(), ob.Leader)
				}
			case leader := <-th.raft.LeaderCh():
				logrus.Infof("[Observer][%s][%d]Leader,%v", th.config.NodeId, th.raft.LastIndex(), leader)
				th.setLeader(leader)
			case <-th.exitChan:
				logrus.Infof("[Observer][%s][%d]Exit", th.config.NodeId, th.raft.LastIndex())
				return
			}
		}
	})

	th.statTicker = common.NewTicker(5*time.Second, func() {
		logrus.Debugf("[%s][%v]%v", th.config.NodeId, th.raft.State(), th.raft.Stats())
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

//非协程安全的
func (th *RaftStore) setLeader(leader bool) {
	logrus.Infof("[%s]setLeader,%v,%v", th.config.NodeId, th.leader, leader)
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

//阻塞的
func (th *RaftStore) Shutdown() {
	if th.stopped {
		return
	}
	th.stopped = true
	if th.raft != nil {
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
	logrus.Debugf("[%s]ReloadNode,%v", th.config.NodeId, nodes)
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
