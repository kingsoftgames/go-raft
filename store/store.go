package store

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/inner"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/common"
	"github.com/golang/protobuf/proto"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
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

type RaftStore struct {
	RaftDir      string
	RaftBindAddr string
	ApiAddr      string
	l            sync.RWMutex

	storeInMem bool
	m          StoreType

	raft *raft.Raft

	OnStateChg  common.SafeEvent
	OnLeaderChg common.SafeEvent
	OnLeader    common.SafeEvent

	transport *raft.NetworkTransport

	peers sync.Map

	runChan common.RunChanType
}

func New(storeInMem bool, raftDir, raftAddr, apiAddr string, runChan common.RunChanType) *RaftStore {
	return &RaftStore{
		RaftDir:      raftDir,
		RaftBindAddr: raftAddr,
		ApiAddr:      apiAddr,
		m:            make(StoreType),
		storeInMem:   storeInMem,
		runChan:      runChan,
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
	go func() {
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
	}()
}
func (th *RaftStore) SetAsync(key string, valueType ValueType, fn func(err error, rsp interface{})) {
	go func() {
		f := th.Set(key, valueType)
		err := f.Error()
		if th.runChan != nil {
			th.runChan <- func() {
				fn(err, f.Response())
			}
		} else {
			fn(err, f.Response())
		}
	}()
}
func (th *RaftStore) DeleteAsync(key string, fn func(err error, rsp interface{})) {
	go func() {
		f := th.Delete(key)
		err := f.Error()
		if th.runChan != nil {
			th.runChan <- func() {
				fn(err, f.Response())
			}
		} else {
			fn(err, f.Response())
		}
	}()
}
func (th *RaftStore) Join(nodeId string, addr string, apiAddr string) error {
	logrus.Infof("Join %s,%s", nodeId, addr)
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
	if err := th.SetApiAddr(addr, apiAddr); err != nil {
		logrus.Infof("SetApiAddr failed,%s", err.Error())
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
	//logrus.Infof("[Store]Apply %d", cmd.Cmd)
	switch cmd.Cmd {
	case inner.ApplyCmd_GET:
		return th.applyGet(cmd.GetGet())
	case inner.ApplyCmd_SET:
		//logrus.Infof("[Store]Apply Set %s,%s", cmd.GetSet().Key, string(cmd.GetSet().Value))
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
	logrus.Infof("Snapshot")
	th.l.RLock()
	defer th.l.RUnlock()
	sf := &storeFsmSnapshot{
		s: make(StoreType),
	}
	sf.s.copyFrom(th.m)
	return sf, nil
}
func (th *RaftStore) Restore(rc io.ReadCloser) error {
	logrus.Infof("Restore")
	o := make(StoreType)
	if err := common.DecodeFromReader(rc, &o); err != nil {
		return err
	}
	th.m = o
	return nil
}

func (th *RaftStore) Open(joinAddr string, localId string) error {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localId)
	//config.Logger = &raftLog{}
	addr, err := net.ResolveTCPAddr("tcp", th.RaftBindAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(th.RaftBindAddr, addr, 3, raftTimeout, os.Stderr)
	if err != nil {
		return err
	}
	th.transport = transport
	snapshot, err := raft.NewFileSnapshotStore(th.RaftDir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return err
	}
	var logStore raft.LogStore
	var stableStore raft.StableStore
	if th.storeInMem {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
	} else {
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(th.RaftDir, "raft.db"))
		if err != nil {
			return fmt.Errorf("new bolt store: %s", err)
		}
		logStore = boltDB
		stableStore = boltDB
	}
	ra, err := raft.NewRaft(config, th, logStore, stableStore, snapshot, transport)
	if err != nil {
		return err
	}
	th.raft = ra
	th.runObserver()
	//joinAddr为空说明第一个启动，一般就作为leader了，同一个cluster中后面的节点不能再以这个模式启动
	if joinAddr == "" {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		if err := ra.BootstrapCluster(configuration).Error(); err != nil {
			logrus.Infof("maybe restart,%s\n", err.Error())
		}
	}
	return nil
}
func (th *RaftStore) SetApiAddr(raftAddr, apiAddr string) error {
	return th.Set(GetApiKey(raftAddr), apiAddr).Error()
}
func (th *RaftStore) GetApiAddr(raftAddr string) (string, error) {
	v, e := th.Get(GetApiKey(raftAddr))
	if v == nil || e != nil {
		return "", e
	}
	var apiAddr string
	err := common.Decode(v.([]byte), &apiAddr)
	return apiAddr, err
}
func (th *RaftStore) runObserver() {
	obchan := make(chan raft.Observation, 1024)
	obsrv := raft.NewObserver(obchan, true, func(o *raft.Observation) bool {
		return true
	})
	th.raft.GetConfiguration()
	th.raft.RegisterObserver(obsrv)
	go func() {
		defer close(obchan)
		terminate := make(chan os.Signal, 1)
		defer close(terminate)
		signal.Notify(terminate, os.Interrupt)
		signal.Notify(terminate, os.Kill)
		//observer,管道大小弄大点，不然投票容易超时，因为block为true时，channel处理不过来会造成投票的channel阻塞

		for {
			select {
			case obs := <-obchan:
				switch obs.Data.(type) {
				case *raft.RequestVoteRequest:
					ob := obs.Data.(*raft.RequestVoteRequest)
					logrus.Infof("[Observer]RequestVoteRequest,%v", *ob)
				case raft.RaftState:
					ob := obs.Data.(raft.RaftState)
					logrus.Debugf("[Observer]RaftState,%v", ob)
					if th.runChan != nil {
						th.runChan <- func() {
							th.OnStateChg.EmitSafe(ob)
						}
					} else {
						th.OnStateChg.EmitSafe(ob)
					}
				case raft.PeerObservation:
					ob := obs.Data.(raft.PeerObservation)
					logrus.Infof("[Observer]PeerObservation,%v", ob)
					if ob.Removed {
						th.peers.Delete(ob.Peer.ID)
					} else {
						th.peers.Store(ob.Peer.ID, ob.Peer)
					}
				case raft.LeaderObservation:
					ob := obs.Data.(raft.LeaderObservation)
					if !th.IsLeader() { //通知leader的addr
						if th.runChan != nil {
							th.runChan <- func() {
								th.OnLeaderChg.EmitSafe(string(ob.Leader))
							}
						} else {
							th.OnLeaderChg.EmitSafe(string(ob.Leader))
						}
					} else {
						if err := th.SetApiAddr(th.RaftBindAddr, th.ApiAddr); err != nil {
							logrus.Infof("SetApiAddr error,%s\n", err.Error())
						} else {
							logrus.Infof("[Observer]SetApiAddr %s , %s\n", th.RaftBindAddr, th.ApiAddr)
						}
					}
					logrus.Infof("[Observer]LeaderObservation,%v", ob)
				}
			case leader := <-th.raft.LeaderCh():
				logrus.Info("[Leader] %v", leader)
				if th.runChan != nil {
					th.runChan <- func() {
						th.OnLeader.EmitSafe(leader)
					}
				} else {
					th.OnLeader.EmitSafe(leader)
				}
			case <-terminate:
				return
			}
		}
	}()
}

func (th *RaftStore) IsLeader() bool {
	return th.raft.State() == raft.Leader
}
func (th *RaftStore) IsFollower() bool {
	return th.raft.State() == raft.Follower
}
func (th *RaftStore) Release() {
	th.transport.Close()
	th.transport = nil
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
