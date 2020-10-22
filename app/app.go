package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/raft"

	"git.shiyou.kingsoft.com/infra/go-raft/common"
	"git.shiyou.kingsoft.com/infra/go-raft/inner"
	"git.shiyou.kingsoft.com/infra/go-raft/store"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"

	_ "github.com/gin-gonic/gin"
	_ "github.com/go-yaml/yaml"
	"github.com/sirupsen/logrus"
)

const (
	VER = "v1.2.0"
)
const (
	tryConnectInterval = 2000 * time.Millisecond
	grpcTimeout        = 5000 * time.Millisecond
	healthInterval     = 2000 * time.Millisecond
	delayStop          = 10000 * time.Millisecond
	stopTimeout        = 5 * 60 * 1000 * time.Millisecond
	updateTimeout      = 1000 * time.Millisecond
	handleTimeout      = 5000 * time.Millisecond
	futureRspTimeout   = 9000 * time.Millisecond

	ResultCodeErr       = -1
	ResultCodeNotLeader = 1
	ResultCodeExists    = 2
)

var (
	errTransfer  = errors.New("trans to not leader node")
	errLeaderCon = errors.New("leader con failed")
	errTimeout   = errors.New("timeout")
)

type IGRpcHandler interface {
	GRpcHandle(*ReplyFuture)
}
type IApp interface {
	IGRpcHandler
	Init(app *MainApp) error
	Release()
	Register(server *grpc.Server)
	OnLeader(bool)
}

func handleContext(runLogic *common.LogicChan, ctx context.Context, timeout time.Duration, h func(error)) {
	if hash := ctx.Value("hash"); hash != nil {
		if s, ok := hash.(string); ok {
			runLogic.HandleWithHash(s, timeout, h)
		} else if i, ok := hash.(int); ok {
			runLogic.Handle(i, timeout, h)
		}
	} else {
		runLogic.Handle(0, timeout, h)
	}
}

type MainApp struct {
	app IApp
	common.GracefulGoFunc
	common.CheckWork
	api    *GRpcService //out for api
	inner  *GRpcService //inner api transfer
	config *common.Configure

	store *store.RaftStore

	//deal application logic
	runLogic common.LogicChan

	//deal inner logic
	innerLogic common.LogicChan

	handler Handler

	http *httpApi

	OnLeaderChg common.SafeEvent

	stopWait sync.WaitGroup

	members    *MemberList
	healthTick *common.Ticker

	watch *common.FileWatch

	timer *common.Timer

	isBoot atomic.Value

	updateTimer *common.Timer

	stopChan      chan struct{}
	stopOtherChan chan struct{}
	stopTimer     *common.Timer
	stopLock      sync.Mutex
	stopped       atomic.Value

	latestVersion atomic.Value

	grpcChan            chan *ReplyFuture
	grpcPrioritizedChan chan *ReplyFuture //加急
	otherChan           chan *ReplyFuture
	sig                 chan struct{}

	debug atomic.Value
}

func NewMainApp(app IApp, exitWait *common.GracefulExit) *MainApp {
	mainApp := &MainApp{
		app:                 app,
		config:              common.NewDefaultConfigure(),
		grpcChan:            make(chan *ReplyFuture, 1024),
		grpcPrioritizedChan: make(chan *ReplyFuture, 64),
		otherChan:           make(chan *ReplyFuture, 16),
		sig:                 make(chan struct{}, 1),
		stopChan:            make(chan struct{}, 1),
		stopOtherChan:       make(chan struct{}, 1),
		members:             &MemberList{},
	}
	mainApp.isBoot.Store(false)
	mainApp.watch = common.NewFileWatch(mainApp)
	if exitWait == nil {
		exitWait = &common.GracefulExit{}
	}
	mainApp.UpdateExitWait(exitWait)
	return mainApp
}
func (th *MainApp) checkCfg() bool {
	if th.config.Bootstrap && th.config.BootstrapExpect > 0 {
		logrus.Errorf("'bootstrap_expect > 0' and 'bootstrap = true' are mutually exclusive")
		return false
	}
	return true
}
func (th *MainApp) setWork(work bool) {
	if work {
		if s := th.stopped.Load(); s != nil && s.(bool) { //已经就让退出模式，不让重新work
			logrus.Debugf("[%s]stopping can not set work", th.config.NodeId)
			return
		}
		th.Work()
		th.api.SetHealth(true)
	} else {
		th.Idle()
		th.api.SetHealth(false)
	}
}
func (th *MainApp) Init(configPath string) int {
	th.config = common.InitConfigureFromFile(configPath)
	if !th.checkCfg() {
		return -1
	}
	th.initDebugConfig()
	common.InitLog(th.config.LogConfig)
	s, _ := json.Marshal(th.config)
	logrus.Infof("Configure : %s", string(s))
	common.InitCodec(th.config.Codec)
	if err := th.app.Init(th); err != nil {
		logrus.Errorf("app Init err,%s", err.Error())
		return -2
	}
	th.Name = th.config.NodeId
	th.runLogic.Init(th.config.RunChanNum, th)
	th.innerLogic.Init(1, th)
	th.config.StoreDir = filepath.Join(th.config.StoreDir, th.config.NodeId)
	th.store = store.New(th.config, nil, th)
	th.store.OnLeader.Add(func(i interface{}) {
		if i.(bool) {
			th.startUpdateTime()
			th.onChgToLeader()
			th.updateLeaderClient(nil)
		} else {
			th.stopUpdateTime()
		}
		th.app.OnLeader(i.(bool))
	})
	th.store.OnLeaderChg.Add(func(i interface{}) {
		th.OnLeaderChg.Emit(i)
		if i != nil && len(i.(string)) > 0 {
			//获取leader节点的连接，如果获取到了，那么设置inner的client为leader的连接
			if m := th.members.GetByRaftAddr(i.(string)); m != nil {
				logrus.Debugf("[%s]Leader Chg %s,%d", th.config.NodeId, i.(string), th.store.GetRaft().LastIndex())
				th.updateLeaderClient(m.Con)
				if m.Con != nil { //如果连接不为空，那么设置可以work
					th.setWork(true)
				} else {
					th.setWork(false)
					logrus.Debugf("[%s]leader not connect Work(%v) ", th.config.NodeId, th.Check())
					return
				}
			}
		} else {
			th.updateLeaderClient(nil)
			//th.setWork(false)
		}
		th.isBoot.Store(true)
	})
	th.store.OnPeerAdd.Add(func(i interface{}) {
		//if th.store.IsLeader() {
		//	th.checkRemoveOldVersionNode()
		//}
	})
	th.members.OnAddEvent.Add(func(i interface{}) {
		leaderAddr := string(th.store.GetRaft().Leader())
		m := i.(*Member)
		logrus.Debugf("OnAddEvent %s , %s", m.RaftAddr, leaderAddr)
		if i.(*Member).RaftAddr == leaderAddr { //如果连接的节点为leader节点，那么设置可以work
			th.setWork(true)
			th.updateLeaderClient(m.Con)
		}
	})
	th.store.OnStateChg.Add(func(i interface{}) {
		th.sigGo()
		switch i.(raft.RaftState) {
		case raft.Leader, raft.Follower:
			th.setWork(true)
			th.isBoot.Store(true)
		case raft.Candidate:
			//th.setWork(false)
			th.isBoot.Store(true)
		default:
			th.setWork(false)
		}
	})
	if err := th.store.Open(th.config.LogConfig.Level, common.NewFileLog(th.config.LogConfig, th.config.NodeId, "raft")); err != nil {
		logrus.Errorf("store open err,%s", err.Error())
		return -3
	}
	th.joinSelf()
	th.inner = New(th.config.InnerAddr, th.store, th, "inner")
	inner.RegisterRaftServer(th.inner.GetGrpcServer(), &RaftServerGRpc{App: th})
	if err := th.inner.Start(); err != nil {
		logrus.Errorf("inner start err,%s", err.Error())
		return -4
	}
	th.inner.SetHealth(true)
	//th.WaitGo()

	th.api = New(th.config.GrpcApiAddr, th.store, th, "api")
	th.app.Register(th.api.GetGrpcServer())
	if err := th.api.Start(); err != nil {
		logrus.Errorf("api start err,%s", err.Error())
		return -5
	}
	th.api.SetHealth(false)
	//th.WaitGo()
	th.Go(th.runOtherRequest)
	th.WaitGo()

	if rst := th.tryJoin(th.config.JoinAddr, true); rst != 0 {
		return -6
	}
	th.handler.Register(th.app)
	th.initHttpApi()
	th.healthTicker()
	th.watchJoinFile()
	th.Work()
	logrus.Infof("[%s]Init finished[%s][%d]", th.config.NodeId, VER, os.Getpid())
	return 0
}
func (th *MainApp) initDebugConfig() {
	DebugTraceFutureLine = th.config.DebugConfig.TraceLine
	if th.config.DebugConfig.PrintIntervalMs > 0 {
		logrus.Infof("[%s]initDebugConfig", th.config.NodeId)
		var cnt, totalTime, t int64
		common.NewTicker(time.Duration(th.config.DebugConfig.PrintIntervalMs)*time.Millisecond, func() {
			logrus.Infof("[%s]%s", th.config.NodeId, GetFutureAve())
			th.PrintQPS()
			_t := time.Now().UnixNano()
			_cnt, _totalTime := th.runLogic.GetCnt()
			if cnt > 0 {
				__cnt := _cnt - cnt
				__totalTime := _totalTime - totalTime
				__t := _t - t
				logrus.Infof("[%s]LastTerm %dms,cnt %d,totalTime %dms, qps %v/s,", th.config.NodeId, __t/1e6, __cnt, __totalTime/1e3, __cnt/(__totalTime/1e6))
			}
			cnt, totalTime, t = _cnt, _totalTime, _t
		})
	}
}
func (th *MainApp) release() {
	defer func() {
		th.Done()
		th.stopWait.Done()
	}()
	th.app.Release()
	logrus.Infof("[%s]release", th.config.NodeId)
}
func (th *MainApp) PrintQPS() {
	qps := th.runLogic.GetQPS()
	if len(qps) > 0 {
		logrus.Infof("[%s]QPS,%s", th.config.NodeId, strings.Join(qps, "\\n"))
	}
}

//wait stop complete
func (th *MainApp) Stopped() {
	th.stopWait.Wait()
}
func (th *MainApp) GRpcHandlePrioritize(f *ReplyFuture) {
	f.prioritized = true
	th.Go(func() {
		th.GRpcHandle(f)
	})
}
func (th *MainApp) GRpcHandle(f *ReplyFuture) {
	if !th.config.DebugConfig.GRpcHandleHash {
		f.AddTimeLine("GRpcHandle")
		f.cnt++
		switch f.cmd {
		case FutureCmdTypeGRpc:
			if f.prioritized {
				th.Go(func() {
					th.grpcPrioritizedChan <- f
				})
			} else {
				//
				switch th.store.GetRaft().State() {
				case raft.Leader:
					th.leaderGRpc(f)
				case raft.Follower:
					if con := th.inner.GetInner(); con != nil {
						th.followerGRpc(f, con)
					} else {
						th.GRpcHandlePrioritize(f)
					}
				case raft.Candidate:
					th.GRpcHandlePrioritize(f)
				case raft.Shutdown:
					if con := th.inner.GetLastInner(); con != nil {
						th.followerGRpc(f, con)
					} else {
						f.response(errors.New("shutdown"))
					}
				}
			}
		default:
			th.Go(func() {
				if f.cnt > 1 {
					time.Sleep(time.Duration(f.cnt) * 5 * time.Millisecond)
				}
				th.otherChan <- f
			})
		}
	} else {
		th.Go(func() {
			f.AddTimeLine("GRpcHandle")
			f.cnt++
			switch f.cmd {
			case FutureCmdTypeGRpc:
				if f.prioritized {
					th.grpcPrioritizedChan <- f
				} else {
					th.grpcChan <- f
				}
			default:
				if f.cnt > 1 {
					time.Sleep(time.Duration(f.cnt) * 5 * time.Millisecond)
				}
				th.otherChan <- f
			}

		})
	}
}
func (th *MainApp) updateLeaderClient(con *InnerCon) {
	th.inner.UpdateClient(con)
	th.sigGo()
}
func (th *MainApp) HttpCall(ctx context.Context, path string, data []byte) ([]byte, error) {
	return th.http.call(ctx, path, data)
}
func (th *MainApp) Start() {
	th.Add()
	th.stopWait.Add(1)
	th.Go(func() {
		th.innerLogic.Start()
	})
	th.Go(th.runGRpcRequest)
	th.Go(func() {
		defer func() {
			th.release()
		}()
		th.runLogic.Start()
	})
}
func (th *MainApp) gracefulShutdown() {
	if s := th.stopped.Load(); s != nil && s.(bool) {
		return
	}
	logrus.Infof("[%s]gracefulShutdown,begin", th.config.NodeId)
	th.preStop()
	th.shutdown()
	logrus.Infof("[%s]gracefulShutdown,end", th.config.NodeId)
}
func (th *MainApp) shutdown() {
	logrus.Infof("[%s]shutdown,begin", th.config.NodeId)
	th.runLogic.Stop()
	th.store.Shutdown()
	th.innerLogic.Stop()
	go func() {
		th.stopChan <- struct{}{}
	}()
	//if th.inner != nil {
	//	th.inner.Stop()
	//	th.inner = nil
	//}
	//if th.api != nil {
	//	th.api.Stop()
	//	th.api = nil
	//}
	//if th.http != nil {
	//	th.http.close()
	//	th.http = nil
	//}
	logrus.Infof("[%s]shutdown,end", th.config.NodeId)
}
func (th *MainApp) preStop() {
	th.Idle()
	th.api.SetHealth(false)
	th.stopUpdateTime()
	th.stopHealthTicker()
	th.watch.Stop()
}
func (th *MainApp) Stop() {
	if s := th.stopped.Load(); s != nil && s.(bool) {
		return
	}
	th.stopLock.Lock()
	defer th.stopLock.Unlock()
	if s := th.stopped.Load(); s != nil && s.(bool) {
		return
	}
	th.stopped.Store(true)
	logrus.Infof("[%s]Stop,begin", th.config.NodeId)
	th.preStop()
	f := NewReplyFuturePrioritized(context.Background(), nil, nil)
	f.cmd = FutureCmdTypeGracefulStop
	th.GRpcHandle(f)
	if f.Error() != nil {
		logrus.Errorf("[%s]GracefulStop,err,%s", th.config.NodeId, f.Error().Error())
	}
	logrus.Infof("[%s]Stop,end", th.config.NodeId)
}

func (th *MainApp) GetStore() *store.RaftStore {
	return th.store
}

func (th *MainApp) initHttpApi() {
	th.http = newHttpApi(th)
	th.http.init(th.config.HttpApiAddr)
}
func (th *MainApp) NewTimer(duration time.Duration, cb func()) *common.Timer {
	return common.NewTimer(duration, func() {
		th.innerLogic.HandleNoHash(0, func(err error) {
			if err != nil {
				cb()
			}
		})
	})
}
func (th *MainApp) tryConnect(addr string, tryCnt *int, rstChan chan int) {
	if tryCnt == nil {
		var cnt = 0
		tryCnt = &cnt
	}
	var rst = 1
	defer func() {
		if rstChan != nil && rst != 1 {
			rstChan <- rst
		}
	}()
	*tryCnt++
	logrus.Infof("[%s]tryConnect %s,%d/%d", th.config.NodeId, addr, *tryCnt, th.config.TryJoinTime)
	g := NewInnerGRpcClient(time.Duration(th.config.ConnectTimeoutMs) * time.Millisecond)
	if err := g.Connect(addr, th.config.NodeId); err != nil {
		logrus.Errorf("Join %s failed,%s", addr, err.Error())
		if *tryCnt >= th.config.TryJoinTime {
			rst = -1
			return
		}
		//TODO  Bug : th.timer cannot shared by all connect
		th.timer = common.NewTimer(time.Duration(*tryCnt)*tryConnectInterval, func() {
			th.timer = nil
			th.tryConnect(addr, tryCnt, rstChan)
		})
		return
	}
	*tryCnt = 0
	//ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs)
	if rsp, err := g.GetClient().JoinRequest(context.Background(), &inner.JoinReq{
		Info: &inner.Member{
			NodeId:    th.config.NodeId,
			RaftAddr:  th.config.RaftAddr,
			InnerAddr: th.config.InnerAddr,
			Ver:       th.config.Ver,
		},
	}); err != nil {
		logrus.Errorf("[%s]JoinRequest error,%s,%s", th.config.NodeId, addr, err.Error())
		rst = -2
		return
	} else {
		if rsp.Result < 0 {
			logrus.Error("[%s]JoinRequest failed,%s,%s", th.config.NodeId, addr, rsp.Message)
			rst = -3
			return
		} else if rsp.Result == ResultCodeExists {
			th.OnlyJoin(rsp.Info)
			logrus.Infof("[%s]JoinRequest exists,%s", th.config.NodeId, addr)
		}
		logrus.Infof("[%s]JoinRequest succeed,%s,%d", th.config.NodeId, addr, rsp.Result)
	}
	rst = 0
}
func (th *MainApp) tryJoin(addrs string, block bool) (rst int) {
	if len(addrs) == 0 {
		return 0
	}
	addrV := strings.Split(addrs, ",")
	var rstChan chan int
	if block {
		rstChan = make(chan int, 1)
		defer close(rstChan)
	}
	for _, addr := range addrV {
		if th.members.GetByGrpcAddr(addr) != nil {
			logrus.Debugf("[%s]tryJoin,already in ,%s", th.config.NodeId, addr)
			continue
		}
		th.GoN(func(p ...interface{}) {
			th.tryConnect(p[0].(string), nil, rstChan)
		}, addr)
		if rstChan != nil {
			rst = <-rstChan
			if rst == 0 {
				break
			}
		}
	}
	return
}
func (th *MainApp) joinSelf() {
	th.members.selfNodeId = th.config.NodeId
	th.Join(&inner.Member{
		NodeId:    th.config.NodeId,
		RaftAddr:  th.config.RaftAddr,
		InnerAddr: th.config.InnerAddr,
		Ver:       th.config.Ver,
	})
	th.updateLatestVersion(th.config.Ver)
}
func (th *MainApp) updateLatestVersion(ver string) {
	v := th.latestVersion.Load()
	if v == nil || v.(string) < ver {
		th.latestVersion.Store(ver)
	}
}
func (th *MainApp) OnlyJoin(info *inner.Member) error {
	return th.members.Add(&Member{
		NodeId:    info.NodeId,
		RaftAddr:  info.RaftAddr,
		InnerAddr: info.InnerAddr,
		Ver:       info.Ver,
		LastIndex: info.LastIndex,
	})
}
func (th *MainApp) joinMemBoot(info *inner.Member) error {
	if r, err := th.addMem(info); r {
		return err
	}
	if th.GetStore().IsLeader() {
		if err := th.GetStore().Join(info.NodeId, info.RaftAddr); err != nil {
			return err
		}
	} else {
		logrus.Infof("[%s]Join,len(%d)", th.config.NodeId, th.members.Len())
		if th.config.BootstrapExpect > 0 && th.members.Len() >= th.config.BootstrapExpect {
			th.members.Foreach(func(member *Member) {
				th.GetStore().AddServer(member.NodeId, member.RaftAddr)
			})
			//开始选举
			if err := th.GetStore().BootStrap(); err != nil {
				logrus.Errorf("[%s] BootStrap,err,%s", th.config.NodeId, err.Error())
				return err
			}
		}
	}
	return nil
}
func (th *MainApp) joinMem(info *inner.Member) (err error) {
	logrus.Infof("[%s]joinMem,%s", th.config.NodeId, info.NodeId)
	if th.store.IsLeader() { //leader
		if info.Ver < th.config.Ver { //小于当前版本
			return fmt.Errorf("member can not less than cur version(%s < %s)", info.Ver, th.config.Ver)
		}
		if _, err = th.addMem(info); err != nil {
			return
		}
		if err = th.store.Join(info.NodeId, info.RaftAddr); err != nil {
			return
		}
		//if info.Ver > th.config.Ver { //大于当前版本，转移leader权
		//	return th.store.GetRaft().LeadershipTransferToServer(raft.ServerID(info.NodeId), raft.ServerAddress(info.RaftAddr)).Error()
		//}
	} else if th.store.IsFollower() {
		con := th.inner.GetInner()
		if con == nil {
			return fmt.Errorf("[%s]node can not work[%v]", th.config.NodeId, th.store.GetRaft().State())
		}
		con.GetRaftClient(func(client inner.RaftClient) {
			if client != nil {
				ctx, _ := context.WithTimeout(context.Background(), grpcTimeout)
				logrus.Debugf("[%s]JoinRequest begin,%s", th.config.NodeId, info.NodeId)
				if rsp, _err := client.JoinRequest(ctx, &inner.JoinReq{
					Info: &inner.Member{
						NodeId:    info.NodeId,
						RaftAddr:  info.RaftAddr,
						InnerAddr: info.InnerAddr,
						Ver:       info.Ver,
					},
				}); _err != nil {
					err = _err
				} else {
					if rsp.Result != 0 {
						err = fmt.Errorf("JoinRsp err %d,%s", rsp.Result, rsp.Message)
					}
				}
			} else {
				err = errLeaderCon
			}
			logrus.Debugf("[%s]JoinRequest end,%s", th.config.NodeId, info.NodeId)
		})
	} else {
		return fmt.Errorf("node can not work (candidate)")
	}
	return
}
func (th *MainApp) addMem(info *inner.Member) (bool, error) {
	if th.members.Get(info.NodeId) != nil {
		th.members.SynMemberToAll(th.config.Bootstrap, th.config.BootstrapExpect)
		return true, nil
	}
	if err := th.members.Add(&Member{
		NodeId:    info.NodeId,
		RaftAddr:  info.RaftAddr,
		InnerAddr: info.InnerAddr,
		Ver:       info.Ver,
	}); err != nil {
		return true, err
	}
	th.updateLatestVersion(info.Ver)
	if th.config.NodeId == info.NodeId { // join self
		return true, nil
	}
	//同步下
	th.members.SynMemberToAll(th.config.Bootstrap, th.config.BootstrapExpect)
	return false, nil
}
func (th *MainApp) Join(info *inner.Member) error {
	if th.isBoot.Load().(bool) { //
		return th.joinMem(info)
	}
	return th.joinMemBoot(info)
}
func (th *MainApp) stopHealthTicker() {
	if th.healthTick != nil {
		th.healthTick.Stop()
		th.healthTick = nil
	}
}
func (th *MainApp) healthTicker() {
	th.healthTick = common.NewTicker(healthInterval, func() {
		a := th
		a.members.Foreach(func(member *Member) {
			if member.Con != nil {
				th.Go(func() {
					member.Con.GetRaftClient(func(client inner.RaftClient) {
						if client != nil {
							ctx, _ := context.WithTimeout(context.Background(), grpcTimeout)
							_, err := client.HealthRequest(ctx, &inner.HealthReq{
								Info: &inner.Member{
									NodeId:    th.config.NodeId,
									RaftAddr:  th.config.RaftAddr,
									InnerAddr: th.config.InnerAddr,
									Ver:       th.config.Ver,
									LastIndex: th.store.GetRaft().LastIndex(),
								},
								SendTime: time.Now().UnixNano(),
							})
							if err != nil {
								logrus.Warnf("[%s]healthTicker(failed) to [%s], %s", th.config.NodeId, member.NodeId, err.Error())
							} else {
								//logrus.Debugf("[%s]healthTicker(ok) to [%s], %d", th.config.NodeId, member.NodeId, (time.Now().UnixNano()-rsp.SendTime)/1e6)
							}
							member.Health(err == nil)
						}
					})
				})
			}
		})
	})
}
func addrEqual(addr1, addr2 string) bool {
	addr1s := strings.Split(addr1, ",")
	addr2s := strings.Split(addr2, ",")
	sort.Strings(addr1s)
	sort.Strings(addr2s)
	return strings.Join(addr1s, ",") == strings.Join(addr2s, ",")
}
func (th *MainApp) trimJoinFile(file string) {
	if len(file) == 0 {
		return
	}
	addrs, err := common.ReadJoinAddr(file)
	logrus.Infof("[%s]watchJoinFile,%s,{%s}", th.config.NodeId, file, addrs)
	if err != nil {
		logrus.Errorf("watchJoinFile,error,%s,%s", file, err.Error())
	} else {
		if !addrEqual(addrs, th.config.JoinAddr) { //if changed
			th.config.JoinFile = addrs
			th.tryJoin(addrs, false)
		}
	}
}
func (th *MainApp) watchJoinFile() {
	th.innerLogic.HandleNoHash(0, func(err error) {
		if err == nil {
			th.trimJoinFile(th.config.JoinFile)
		}
	})
	if len(th.config.JoinFile) > 0 {
		th.watch.Add(th.config.JoinFile, func(s string) {
			th.innerLogic.HandleNoHash(0, func(err error) {
				if err == nil {
					th.trimJoinFile(s)
				}
			})
		})
	}
	th.watch.Start()
}

func (th *MainApp) checkUpdate() {
	if !th.store.IsLeader() {
		return
	}
	var newLeader *raft.Server
	nodes := th.store.GetNodes()
	latestVer := th.latestVersion.Load().(string)
	for _, node := range nodes {
		mem := th.members.Get(string(node.ID))
		if mem == nil {
			logrus.Errorf("[%s]can not found %s", th.config.NodeId, node.ID)
			continue
		}
		if mem.Ver > th.config.Ver {
			newLeader = &node
		}
		if mem.Ver < latestVer && node.Suffrage == raft.Voter && mem.NodeId != th.config.NodeId { //版本低的移除选举权（不能移除自己）
			if err := th.store.GetRaft().DemoteVoter(node.ID, 0, 0).Error(); err != nil {
				logrus.Errorf("[%s]DemoteVoter %s err,%s", th.config.NodeId, node.ID, err.Error())
			} else {
				logrus.Infof("[%s]DemoteVoter %s", th.config.NodeId, node.ID)
			}
			newLeader = nil
		}
	}
	if newLeader != nil {
		if err := th.store.GetRaft().LeadershipTransferToServer(newLeader.ID, newLeader.Address).Error(); err != nil {
			logrus.Errorf("[%s]LeaderTransfer to %s,err,%s", th.config.NodeId, newLeader.ID, err.Error())
		} else {
			logrus.Infof("[%s]LeaderTransfer to %s", th.config.NodeId, newLeader.ID)
			return
		}
	}
	th.checkRemoveOldVersionNode()
	th.updateTimer = nil
	th.startUpdateTime()
}
func (th *MainApp) transferLeader(newLeader *raft.Server) (err error) {
	logrus.Infof("[%s]transferLeader", th.config.NodeId)
	if th.store.GetRaft().State() == raft.Leader {
		if err = th.store.GetRaft().DemoteVoter(raft.ServerID(th.config.NodeId), 0, 0).Error(); err == nil {
			//if newLeader != nil {
			//	if err = th.store.GetRaft().LeadershipTransferToServer(newLeader.ID, newLeader.Address).Error(); err != nil {
			//		logrus.Errorf("[%s]transferLeader,LeadershipTransferToServer,err,%s", th.config.NodeId, err.Error())
			//	}
			//} else {
			//	if err = th.store.GetRaft().LeadershipTransfer().Error(); err != nil {
			//		logrus.Errorf("[%s]transferLeader,LeadershipTransfer,err,%s", th.config.NodeId, err.Error())
			//	}
			//}
		} else {
			logrus.Errorf("[%s]transferLeader,DemoteVoter,err,%s", th.config.NodeId, err.Error())
		}
	}
	return
}
func (th *MainApp) startUpdateTime() {
	if th.updateTimer == nil {
		th.updateTimer = th.NewTimer(updateTimeout, func() {
			th.checkUpdate()
		})
	}
}
func (th *MainApp) stopUpdateTime() {
	if th.updateTimer != nil {
		th.updateTimer.Stop()
		th.updateTimer = nil
	}
}

//cur node upgrade to leader
func (th *MainApp) onChgToLeader() {
	logrus.Infof("[%s]onChgToLeader,last index,%d", th.config.NodeId, th.store.GetRaft().LastIndex())
}

//only run on leader node
func (th *MainApp) checkRemoveOldVersionNode() {
	nodes := th.store.GetNodes()
	oldMem := make([]*Member, 0)
	for _, node := range nodes {
		mem := th.members.Get(string(node.ID))
		if mem == nil {
			logrus.Errorf("[%s]can not found %s", th.config.NodeId, node.ID)
			continue
		}
		if mem.Ver < th.config.Ver {
			oldMem = append(oldMem, mem)
		}
	}
	removeNodes := len(nodes) - th.config.BootstrapExpect
	for i := 0; i < removeNodes && i < len(oldMem); i++ { //移除多余的旧版本node
		mem := oldMem[i]
		if mem.NodeId == th.config.NodeId {
			continue
		}
		th.removeServer(mem.NodeId)
	}
}
func (th *MainApp) removeServer(nodeId string) error {
	if err := th.store.GetRaft().RemoveServer(raft.ServerID(nodeId), 0, 0).Error(); err != nil {
		logrus.Errorf("[%s]removeServer err,%s,%s", th.config.NodeId, nodeId, err.Error())
		return err
	} else {
		logrus.Infof("[%s]removeServer successful,%s,%v", th.config.NodeId, nodeId, th.store.GetNodes())
		th.members.LeaveToAll(nodeId)
		th.members.Remove(nodeId)
	}
	return nil
}
func (th *MainApp) removeMember(nodeId string) error {
	if nodeId == th.config.NodeId { //移除自己
		th.gracefulShutdown()
	} else { //移除其他节点
		removed := th.members.Remove(nodeId)
		logrus.Infof("[%s]removeMember finished,%s,%v", th.config.NodeId, nodeId, removed)
	}
	return nil
}

func (th *MainApp) leaderGRpc(f *ReplyFuture) {
	f.AddTimeLine("leaderGRpc")
	if f.prioritized {
		logrus.Warnf("[%s]leaderGRpc,%v", th.config.NodeId, f.req)
	}
	h := func(err error) {
		if err != nil {
			logrus.Warnf("[%s]leaderGRpcHandle,err,%s,%v", th.config.NodeId, err.Error(), f.req)
			f.response(err)
			return
		}
		f.AddTimeLine("leaderGRpcHandle")
		if f.prioritized {
			logrus.Warnf("[%s]leaderGRpc,%v", th.config.NodeId, f.req)
		}
		message := f.req.(protoreflect.ProtoMessage)
		method := common.GetHandleFunctionName(message)
		if _, err := th.handler.Handle(method, []reflect.Value{reflect.ValueOf(th.app), reflect.ValueOf(f.req), reflect.ValueOf(f.rsp), reflect.ValueOf(&f.rspFuture)}); err != nil {
			f.response(err)
			return
		}
		if f.rspFuture.Err != nil {
			logrus.Error("handle %s err,%s", method, f.rspFuture.Err.Error())
		}
		if f.rspFuture.Futures.Len() == 0 {
			f.response(f.rspFuture.Err)
		} else { //说明有数据Apply，等待Apply成功
			if err := f.rspFuture.Futures.Error(); err != nil {
				if err == raft.ErrLeadershipLost ||
					err == raft.ErrLeadershipTransferInProgress ||
					err == raft.ErrEnqueueTimeout ||
					err == raft.ErrNotLeader { //
					logrus.Warnf("[%s]ApplyLog warn,%s,%v", th.config.NodeId, err.Error(), f.req)
					if f.trans {
						logrus.Warnf("[%s]leaderGRpc trans back,%v", th.config.NodeId, f.req)
						f.response(errTransfer)
						return
					}
					//Back to grpc chan
					f.rspFuture.Clear()
					th.GRpcHandlePrioritize(f)
					return
				} else {
					logrus.Errorf("[%s]ApplyLog error,%s", th.config.NodeId, err.Error())
				}
				f.response(err)
			} else {
				if f.prioritized {
					logrus.Warnf("[%s]leaderGRpc result,%v", th.config.NodeId, f.req)
				}
				f.AddTimeLine("Response")
				f.response(nil)
			}
		}
	}
	if th.runLogic.CanGo() {
		handleContext(&th.runLogic, f.ctx, handleTimeout, h)
	} else {
		f.response(fmt.Errorf("node are stopping"))
	}
}

/*
如果followerGRpc不单独协程，那么trace时间会主要耗在GRpcHandle ==> followerGRpc路径上
如果followerGRpc单独协程，那么trace时间主要耗在followerGRpc ==> TransGrpcRequest 或者leaderGRpc ==> leaderGRpcHandle路径上
压力特别大是，trace时间会在网络传输上耗时（猜测因为测试是在同一台机器上进行，拆分机器估计会没有该问题）
TODO followerGRpc不单独协程时可以做超时检测，免得把超时的请求发到了leader节点上
*/
func (th *MainApp) followerGRpc(f *ReplyFuture, con *InnerCon) {
	if f.trans {
		logrus.Warnf("[%s]followerGRpc trans,%v", th.config.NodeId, f.req)
		f.response(errTransfer)
		return
	}
	logrus.Debugf("[%s]followerGRpc,%d,%v", th.config.NodeId, f.cnt, f.req)
	t := time.Now().UnixNano()
	defer func() {
		if dif := time.Now().UnixNano() - t; dif > 1e9 {
			logrus.Errorf("[%s]followerGRpc[%v]%v,%v,%v", th.config.NodeId, dif, f.req, con.addr, f.Timeline)
		}
	}()
	f.AddTimeLine("followerGRpc")
	if f.prioritized {
		logrus.Warnf("[%s]followerGRpc,%v,%v", th.config.NodeId, th.store.GetRaft().State(), f.req)
	}

	if th.store.GetRaft().State() == raft.Leader {
		th.GRpcHandlePrioritize(f)
		return
	}
	req := &inner.TransGrpcReq{
		Name:     common.GetHandleFunctionName(f.req.(protoreflect.ProtoMessage)),
		Timeline: make([]*inner.TimeLineUnit, 0),
	}
	if hash := f.ctx.Value("hash"); hash != nil {
		req.Hash = hash.(string)
	}
	req.Data, _ = common.Encode(f.req)
	req.Prioritized = f.prioritized
	for _, line := range f.Timeline {
		req.Timeline = append(req.Timeline, &inner.TimeLineUnit{
			Tag: line.Tag,
			T:   line.T,
		})
	}
	ctx, _ := context.WithTimeout(context.Background(), grpcTimeout)
	con.GetRaftClient(func(client inner.RaftClient) {
		if client == nil {
			f.response(fmt.Errorf("node can not work"))
			return
		}
		rsp, err := client.TransGrpcRequest(ctx, req)
		if err != nil {
			f.response(err)
			return
		} else {
			f.Timeline = nil
			for _, line := range rsp.Timeline {
				f.AddTimelineObj(TimelineInfo{
					Tag: line.Tag,
					T:   line.T,
				})
			}
			f.AddTimeLine("followerGRpcTransLeader")
			if rsp.Back { //退回
				logrus.Warnf("[%s]followerGRpc back,%v", th.config.NodeId, f.req)
				//Back to grpc chan
				f.rspFuture.Clear()
				th.GRpcHandlePrioritize(f)
			} else {
				f.response(common.Decode(rsp.Data, f.rsp))
			}
		}
	})
}
func (th *MainApp) leaderJoin(f *ReplyFuture) {
	info := f.req.(*inner.JoinReq).Info
	logrus.Infof("[%s]leaderJoin,%s", th.config.NodeId, info.NodeId)
	if info.Ver < th.config.Ver { //小于当前版本
		f.response(fmt.Errorf("member can not less than cur version(%s < %s)", info.Ver, th.config.Ver))
		return
	}
	if _, err := th.addMem(info); err != nil {
		f.response(err)
		return
	}
	if err := th.store.Join(info.NodeId, info.RaftAddr); err != nil {
		if err == raft.ErrNotLeader || err == raft.ErrLeadershipTransferInProgress {
			th.GRpcHandle(f)
		} else {
			f.response(err)
		}
	} else {
		f.response(nil)
	}

}
func (th *MainApp) followJoin(f *ReplyFuture) {
	info := f.req.(*inner.JoinReq).Info
	logrus.Infof("[%s]followJoin[%d],%s", th.config.NodeId, f.cnt, info.NodeId)
	defer logrus.Infof("[%s]followJoin finished,%s", th.config.NodeId, info.NodeId)
	if th.isBoot.Load().(bool) {
		if th.members.Get(info.NodeId) != nil {
			f.rsp.(*inner.JoinRsp).Result = ResultCodeExists
			f.rsp.(*inner.JoinRsp).Info = &inner.Member{
				NodeId:    th.config.NodeId,
				RaftAddr:  th.config.RaftAddr,
				InnerAddr: th.config.InnerAddr,
				Ver:       th.config.Ver,
				LastIndex: th.store.GetRaft().LastIndex(),
			}
			f.response(nil)
			return
		}
		con := th.inner.GetInner()
		if con == nil {
			th.GRpcHandle(f)
			return
		}
		con.GetRaftClient(func(client inner.RaftClient) {
			if client != nil {
				ctx, _ := context.WithTimeout(context.Background(), grpcTimeout)
				logrus.Debugf("[%s]JoinRequest begin,%s", th.config.NodeId, info.NodeId)
				rsp, err := client.JoinRequest(ctx, &inner.JoinReq{
					Info: &inner.Member{
						NodeId:    info.NodeId,
						RaftAddr:  info.RaftAddr,
						InnerAddr: info.InnerAddr,
						Ver:       info.Ver,
					},
				})
				if err == nil {
					*f.rsp.(*inner.JoinRsp) = *rsp
				}
				f.response(err)
			} else {
				f.response(errLeaderCon)
			}
			logrus.Debugf("[%s]JoinRequest end,%s", th.config.NodeId, info.NodeId)
			return
		})
	} else {
		if r, err := th.addMem(info); r {
			f.response(err)
			return
		}
		logrus.Infof("[%s]Join,len(%d)", th.config.NodeId, th.members.Len())
		if th.config.BootstrapExpect > 0 && th.members.Len() >= th.config.BootstrapExpect {
			th.members.Foreach(func(member *Member) {
				th.GetStore().AddServer(member.NodeId, member.RaftAddr)
			})
			//开始选举
			if err := th.GetStore().BootStrap(); err != nil {
				logrus.Errorf("[%s] BootStrap,err,%s", th.config.NodeId, err.Error())
				if err == raft.ErrCantBootstrap {
					th.GRpcHandle(f)
				} else {
					f.response(err)
				}
			} else {
				f.response(nil)
			}
		} else {
			f.response(nil)
		}
	}
}
func (th *MainApp) leaderGracefulStop(f *ReplyFuture) {
	logrus.Infof("[%s]leaderGracefulStop", th.config.NodeId)
	if l := len(th.store.GetNodes()); l > 1 {
		if err := th.transferLeader(nil); err == nil || err == raft.ErrNotLeader {
			th.GRpcHandle(f)
		} else {
			logrus.Errorf("[%s]leaderGracefulStop,err,%s", th.config.NodeId, err.Error())
			th.GRpcHandle(f)
		}
	} else {
		logrus.Infof("[%s]leaderGracefulStop,%d", th.config.NodeId, l)
		th.shutdown()
		f.response(nil)
	}
	//if len(th.store.GetNodes()) > 0 {
	//	if err := th.removeServer(th.config.NodeId); err == raft.ErrNotLeader {
	//		th.GRpcHandle(f)
	//		return
	//	}
	//}
	//th.shutdown()
	//f.response(nil)
}
func (th *MainApp) followGracefulStop(f *ReplyFuture) {
	if f.cnt < 10 || f.cnt%10 == 0 {
		logrus.Infof("[%s]followGracefulStop[%d]", th.config.NodeId, f.cnt)
	}
	if th.isBoot.Load().(bool) {
		if l := len(th.store.GetNodes()); l <= 1 {
			logrus.Debugf("[%s]followGracefulStop[%d],%d", th.config.NodeId, f.cnt, l)
			th.shutdown()
			f.response(nil)
			return
		}
		con := th.inner.GetInner()
		if con == nil {
			th.GRpcHandle(f)
			return
		}
		con.GetRaftClient(func(client inner.RaftClient) {
			if client != nil {
				ctx, _ := context.WithTimeout(context.Background(), grpcTimeout)
				rsp, err := client.ExitRequest(ctx, &inner.ExitReq{
					NodeId: th.config.NodeId,
				})
				if err != nil {
					logrus.Errorf("[%s]followGracefulStop boot,err,%s", th.config.NodeId, err.Error())
				} else if rsp.Result == ResultCodeNotLeader {
					logrus.Debugf("[%s]followGracefulStop boot,rollback", th.config.NodeId)
					th.GRpcHandle(f)
					return
				}
			}
			th.shutdown()
			f.response(nil)
		})
	} else {
		th.members.Foreach(func(member *Member) {
			if member.NodeId != th.config.NodeId {
				member.Con.GetRaftClient(func(client inner.RaftClient) {
					if client != nil {
						ctx, _ := context.WithTimeout(context.Background(), grpcTimeout)
						_, err := client.ExitRequest(ctx, &inner.ExitReq{
							NodeId: th.config.NodeId,
						})
						if err != nil {
							logrus.Errorf("[%s]followGracefulStop not boot,err,%s,%s", th.config.NodeId, member.NodeId, err.Error())
						}
					}
				})
			}
		})
		th.shutdown()
		f.response(nil)
	}
}

func (th *MainApp) candidateGracefulStop(f *ReplyFuture) {
	logrus.Infof("[%s]candidateGracefulStop", th.config.NodeId)
	if len(th.store.GetNodes()) == 0 {
		th.shutdown()
		f.response(nil)
	} else {
		th.GRpcHandle(f)
		return
	}
}
func (th *MainApp) leaderExit(f *ReplyFuture) {
	nodeId := f.req.(*inner.ExitReq).NodeId
	logrus.Infof("[%s]leaderExit,%s", th.config.NodeId, nodeId)
	f.response(th.removeServer(nodeId))
}
func (th *MainApp) followerExit(f *ReplyFuture) {
	nodeId := f.req.(*inner.ExitReq).NodeId
	logrus.Infof("[%s]followerExit,%s", th.config.NodeId, nodeId)
	if th.isBoot.Load().(bool) {
		f.response(raft.ErrNotLeader)
	} else {
		th.members.Remove(nodeId)
		f.response(nil)
	}
}
func (th *MainApp) allRemove(f *ReplyFuture) {
	nodeId := f.req.(*inner.RemoveMemberReq).NodeId
	logrus.Infof("[%s]allRemove,%s", th.config.NodeId, nodeId)
	defer logrus.Infof("[%s]allRemove finished,%s", th.config.NodeId, nodeId)
	f.response(th.removeMember(nodeId))
}
func (th *MainApp) allSynMember(f *ReplyFuture) {
	req := f.req.(*inner.SynMemberReq)
	logrus.Infof("[%s]allSynMember,%d,%v,%v", th.config.NodeId, req.BootstrapExpect, req.Bootstrap, req.Mem)
	defer logrus.Infof("[%s]allSynMember finished,%d,%v,%v", th.config.NodeId, req.BootstrapExpect, req.Bootstrap, req.Mem)

	th.config.Bootstrap = req.Bootstrap
	th.config.BootstrapExpect = int(req.BootstrapExpect)
	for _, m := range req.Mem {
		mem := th.members.Get(m.NodeId)
		if mem != nil {
			continue
		}
		if err := th.members.Add(&Member{
			NodeId:    m.NodeId,
			RaftAddr:  m.RaftAddr,
			InnerAddr: m.InnerAddr,
			Ver:       m.Ver,
			LastIndex: m.LastIndex,
		}); err != nil {
			logrus.Errorf("[%s]allSynMember,Add,error,%s,%s", th.config.NodeId, m.NodeId, err.Error())
		}
		th.updateLatestVersion(m.Ver)
	}
	f.response(nil)
}
func (th *MainApp) sigGo() {
	go func() {
		th.sig <- struct{}{}
	}()
}
func (th *MainApp) runGRpcRequest() {
	logrus.Infof("[%s]runGRpcRequest,%d", th.config.NodeId, common.GoID())
	defer func() {
		if err := recover(); err != nil {
			var buf [4096]byte
			n := runtime.Stack(buf[:], false)
			logrus.Debugf("crash %s", string(buf[:n]))
		}
		logrus.Infof("[%s]runGRpcRequest stop", th.config.NodeId)
		fmt.Printf("[%s]consul runGRpcRequest stop", th.config.NodeId)
		if th.inner != nil {
			th.inner.Stop()
			th.inner = nil
		}
		if th.api != nil {
			th.api.Stop()
			th.api = nil
		}
		if th.http != nil {
			th.http.close()
			th.http = nil
		}
		close(th.stopOtherChan)
	}()
	realStop := make(chan struct{}, 1)
	for {
		//if th.debug.Load() != nil {
		//	logrus.Debugf("[%s]runGRpcRequest[%v][%v]", th.config.NodeId, th.store.GetRaft().State(), time.Now().UnixNano()-t)
		//	t = time.Now().UnixNano()
		//}
		stopFunc := func() {
			logrus.Infof("[%s]begin realStop timeout", th.config.NodeId)
			common.NewTimer(delayStop, func() {
				logrus.Infof("[%s]realStop", th.config.NodeId)
				realStop <- struct{}{}
			})
		}
		switch th.store.GetRaft().State() {
		case raft.Leader:
			select {
			case f := <-th.grpcPrioritizedChan:
				th.leaderGRpc(f)
			case f := <-th.grpcChan:
				th.leaderGRpc(f)
			case <-th.stopChan:
				stopFunc()
			case <-realStop:
				logrus.Infof("[%s]rcv realStop", th.config.NodeId)
				return
			case <-th.sig:
				break
			}
		case raft.Follower:
			if con := th.inner.GetInner(); con != nil {
				select {
				case f := <-th.grpcPrioritizedChan:
					th.followerGRpc(f, con)
				case f := <-th.grpcChan:
					th.followerGRpc(f, con)
				case <-th.stopChan:
					stopFunc()
				case <-realStop:
					logrus.Infof("[%s]rcv realStop", th.config.NodeId)
					return
				case <-th.sig:
					break
				}
			} else {
				select {
				case <-th.stopChan:
					logrus.Infof("[%s]follower nil", th.config.NodeId)
					stopFunc()
				case <-realStop:
					logrus.Infof("[%s]rcv realStop", th.config.NodeId)
					return
				case <-th.sig:
					break
				}
			}
		case raft.Candidate:
			select {
			case <-th.stopChan:
				stopFunc()
			case <-realStop:
				logrus.Infof("[%s]rcv realStop", th.config.NodeId)
				return
			case <-th.sig:
				break
			}
		case raft.Shutdown:
			if con := th.inner.GetLastInner(); con != nil {
				select {
				case f := <-th.grpcPrioritizedChan:
					th.followerGRpc(f, con)
				case f := <-th.grpcChan:
					th.followerGRpc(f, con)
				case <-th.stopChan:
					stopFunc()
				case <-realStop:
					logrus.Infof("[%s]shutdown not nil", th.config.NodeId)
					return
				case <-th.sig:
					break
				}
			} else {
				select {
				case <-th.stopChan:
					stopFunc()
				case <-realStop:
					logrus.Infof("[%s]shutdown nil", th.config.NodeId)
					return
				case <-th.sig:
					break
				}
			}
		}
	}

}

func (th *MainApp) runOtherRequest() {
	logrus.Infof("[%s]runOtherRequest,%d", th.config.NodeId, common.GoID())
	defer func() {
		logrus.Infof("[%s]runOtherRequest stop", th.config.NodeId)
	}()
	for {
		switch th.store.GetRaft().State() {
		case raft.Leader:
			select {
			case f := <-th.otherChan:
				switch f.cmd {
				case FutureCmdTypeJoin:
					th.Go(func() {
						th.leaderJoin(f)
					})
				case FutureCmdTypeRemove:
					th.Go(func() {
						th.allRemove(f)
					})
				case FutureCmdTypeSynMember:
					th.Go(func() {
						th.allSynMember(f)
					})
				case FutureCmdTypeGracefulStop:
					th.Go(func() {
						th.leaderGracefulStop(f)
					})
				case FutureCmdTypeExit:
					th.Go(func() {
						th.leaderExit(f)
					})
				default:
					logrus.Errorf("[%s]lost deal cmd %v", th.config.NodeId, f.cmd)
				}
			case <-th.stopOtherChan:
				return
			}
		case raft.Follower:
			select {
			case f := <-th.otherChan:
				switch f.cmd {
				case FutureCmdTypeJoin:
					th.Go(func() {
						th.followJoin(f)
					})
				case FutureCmdTypeRemove:
					th.Go(func() {
						th.allRemove(f)
					})
				case FutureCmdTypeSynMember:
					th.Go(func() {
						th.allSynMember(f)
					})
				case FutureCmdTypeGracefulStop:
					th.Go(func() {
						th.followGracefulStop(f)
					})
				case FutureCmdTypeExit:
					th.Go(func() {
						th.followerExit(f)
					})
				default:
					logrus.Errorf("[%s]lost deal cmd %v", th.config.NodeId, f.cmd)
				}
			case <-th.stopOtherChan:
				return
			}
		case raft.Candidate:
			select {
			case <-th.stopOtherChan:
				return
			default:
				break
			}
		case raft.Shutdown:
			return
		}
	}

}

var appsName = map[string]reflect.Type{}

func RegisterApp(app interface{}) {
	t := reflect.TypeOf(app)
	n := t.Name()
	appsName[n] = t
}
func CreateApp(name string) IApp {
	t, ok := appsName[name+"App"]
	if !ok {
		return nil
	}
	app := reflect.New(t).Interface().(IApp)
	return app
}
func createAllApp() []IApp {
	apps := make([]IApp, 0)
	for _, t := range appsName {
		apps = append(apps, reflect.New(t).Interface().(IApp))
	}
	return apps
}
