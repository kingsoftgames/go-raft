package app

import (
	"context"
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
	tryConnectIntervalMs = 2000
	grpcTimeoutMs        = 2000
	healthIntervalMs     = 2000
	delayStopMs          = 10000
	stopTimeoutMs        = 5 * 60 * 1000
	updateTimeoutMs      = 1000
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

func handleContext(runLogic *common.LogicChan, ctx context.Context, h func()) {
	if hash := ctx.Value("hash"); hash != nil {
		if s, ok := hash.(string); ok {
			runLogic.HandleWithHash(s, h)
		} else if i, ok := hash.(int); ok {
			runLogic.Handle(i, h)
		}
	} else {
		runLogic.Handle(0, h)
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

	stopTimer *common.Timer

	latestVersion atomic.Value
}

func NewMainApp(app IApp, exitWait *common.GracefulExit) *MainApp {
	mainApp := &MainApp{
		app:     app,
		config:  common.NewDefaultConfigure(),
		members: &MemberList{},
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
func (th *MainApp) Init(configPath string) int {
	th.config = common.InitConfigureFromFile(configPath)
	if !th.checkCfg() {
		return -1
	}
	common.InitLog(th.config.LogConfig)
	common.InitCodec(th.config.Codec)
	if err := th.app.Init(th); err != nil {
		logrus.Errorf("app Init err,%s", err.Error())
		return -2
	}
	th.Name = th.config.NodeId
	th.runLogic.Init(runtime.NumCPU()-1, th)
	th.innerLogic.Init(1, th)
	th.config.StoreDir = filepath.Join(th.config.StoreDir, th.config.NodeId)
	th.store = store.New(th.config, nil, th)
	th.store.OnLeader.Add(func(i interface{}) {
		if i.(bool) {
			th.startUpdateTime()
			th.onChgToLeader()
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
				th.inner.UpdateClient(m.Con)
				if m.Con != nil { //如果连接不为空，那么设置可以work
					th.Work()
				} else {
					th.Idle() //如果连接为空，那么设置不可以work
					logrus.Debugf("[%s]leader not connect Work(%v) ", th.config.NodeId, th.Check())
				}
			}
			th.Work()
			th.api.SetHealth(true)
		} else {
			th.Idle()
			th.api.SetHealth(false)
		}
		th.isBoot.Store(true)
	})
	th.store.OnPeerAdd.Add(func(i interface{}) {
		//if th.store.IsLeader() {
		//	th.checkRemoveOldVersionNode()
		//}
	})
	//th.members.OnConEvent.Add(func(i interface{}) {
	//	leaderAddr := string(th.store.GetRaft().Leader())
	//	logrus.Debugf("OnConEvent %s , %s", i.(*Member).RaftAddr, leaderAddr)
	//	if i.(*Member).RaftAddr == leaderAddr { //如果连接的节点为leader节点，那么设置可以work
	//		th.Work()
	//		logrus.Debugf("[%s]connect Work(%v)", th.config.NodeId, th.Check())
	//	}
	//})
	th.store.OnStateChg.Add(func(i interface{}) {
		switch i.(raft.RaftState) {
		case raft.Leader, raft.Follower:
			th.Work()
			th.api.SetHealth(true)
			th.isBoot.Store(true)
		default:
			th.Idle()
			th.api.SetHealth(false)
		}
	})
	if err := th.store.Open(th.config.LogConfig.Level, common.NewFileLog(th.config.LogConfig, th.config.NodeId, "raft")); err != nil {
		logrus.Errorf("store open err,%s", err.Error())
		return -3
	}
	th.joinSelf()
	th.inner = New(th.config.InnerAddr, th.config.ConnectTimeoutMs, th.store, th)
	inner.RegisterRaftServer(th.inner.GetGrpcServer(), &RaftServerGRpc{App: th})
	if err := th.inner.Start(); err != nil {
		logrus.Errorf("inner start err,%s", err.Error())
		return -4
	}
	th.inner.SetHealth(true)
	th.WaitGo()

	th.api = New(th.config.GrpcApiAddr, th.config.ConnectTimeoutMs, th.store, th)
	th.app.Register(th.api.GetGrpcServer())
	if err := th.api.Start(); err != nil {
		logrus.Errorf("api start err,%s", err.Error())
		return -5
	}
	th.api.SetHealth(false)
	th.WaitGo()

	if rst := th.tryJoin(th.config.JoinAddr, true); rst != 0 {
		return -6
	}
	th.handler.Register(th.app)
	th.initHttpApi()
	th.healthTicker()
	th.watchJoinFile()
	th.Work()
	logrus.Infof("[%s]Init finished", th.config.NodeId)
	return 0
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
	for _, qps := range th.runLogic.GetQPS() {
		if len(qps) > 0 {
			logrus.Debugf(qps)
		}
	}
}

//wait stop complete
func (th *MainApp) Stopped() {
	th.stopWait.Wait()
}
func (th *MainApp) GRpcHandle(f *ReplyFuture) {
	if !th.Check() { //不能工作状态
		f.response(fmt.Errorf("node can not work"))
		return
	}
	leader := func() {
		h := func() {
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
				//go func() {
				if err := f.rspFuture.Futures.Error(); err != nil {
					f.response(err)
				} else {
					f.response(nil)
				}
				//}()
			}
		}
		if th.runLogic.CanGo() {
			handleContext(&th.runLogic, f.ctx, h)
		} else {
			f.response(fmt.Errorf("node are stopping"))
		}
	}
	if th.store.IsLeader() {
		leader()
	} else if th.store.IsFollower() {
		if readOnly := f.ctx.Value("readOnly"); readOnly != nil && readOnly.(bool) {
			leader()
			return
		}
		//转发给leader
		th.Go(func() {
			req := &inner.TransGrpcReq{
				Name: common.GetHandleFunctionName(f.req.(protoreflect.ProtoMessage)),
			}
			req.Data, _ = common.Encode(f.req)
			ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
			con := th.inner.GetInner()
			if con == nil {
				f.response(fmt.Errorf("node can not work"))
				return
			}
			con.GetRaftClient(func(client inner.RaftClient) {
				if client == nil {
					f.response(fmt.Errorf("node can not work"))
					return
				}
				rsp, err := client.TransGrpcRequest(ctx, req)
				if err != nil {
					f.response(err)
					return
				}
				f.response(common.Decode(rsp.Data, f.rsp))
			})
		})
	} else {
		f.response(fmt.Errorf("invalid node Candidate"))
	}
}
func (th *MainApp) HttpCall(ctx context.Context, path string, data []byte) ([]byte, error) {
	return th.http.call(ctx, path, data)
}

//非leader调用grpc Client的接口转发到leader处理
func (th *MainApp) GRpcCall(ctx context.Context, name string, data []byte) ([]byte, error) {
	hd := th.handler.GetHandlerValue(name)
	if hd == nil {
		return nil, fmt.Errorf("not found method,%s", name)
	}
	req := reflect.New(hd.paramReq)
	rsp := reflect.New(hd.paramRsp)
	if err := common.Decode(data, req.Interface()); err != nil {
		return nil, err
	}
	rtv := &HandlerRtv{}
	future := NewHttpReplyFuture()
	h := func() {
		hd.fn.Call([]reflect.Value{reflect.ValueOf(th.app), req, rsp, reflect.ValueOf(rtv)})
		if rtv.Futures.Len() != 0 {
			if err := rtv.Futures.Error(); err != nil {
				logrus.Errorf("Future err,%s", err.Error())
			}
		}
		future.Done()
	}
	handleContext(&th.runLogic, ctx, h)
	future.Wait()
	return common.Encode(rsp.Interface())
}
func (th *MainApp) Start() {
	th.Add()
	th.stopWait.Add(1)
	th.Go(func() {
		th.innerLogic.Start()
	})
	th.Go(func() {
		defer func() {
			th.release()
		}()
		th.runLogic.Start()
	})
}
func (th *MainApp) DelayStop() {
	logrus.Infof("[%s]DelayStop,begin", th.config.NodeId)
	th.Idle()
	th.stopUpdateTime()
	th.stopHealthTicker()
	th.watch.Stop()
	th.store.Shutdown()
	th.stopTimer = th.NewTimer(time.Duration(delayStopMs)*time.Millisecond, func() {
		th.Stop()
		th.NewTimer(time.Duration(stopTimeoutMs)*time.Millisecond, func() {
			logrus.Errorf("[%s]DelayStop timeout,forced exit", th.config.NodeId)
			os.Exit(0)
		})
	})
}
func (th *MainApp) Stop() {
	logrus.Infof("[%s]Stop,begin", th.config.NodeId)
	th.Idle()
	//first stop runLogic , make sure all running logic deal over
	if th.timer != nil {
		th.timer.Stop()
		th.timer = nil
	}
	th.stopUpdateTime()
	th.stopHealthTicker()
	th.watch.Stop()
	th.runLogic.Stop()
	th.store.Shutdown()
	th.innerLogic.Stop()
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
		th.innerLogic.HandleNoHash(cb)
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
	g := NewInnerGRpcClient(th.config.ConnectTimeoutMs)
	if err := g.Connect(addr, th.config.NodeId); err != nil {
		logrus.Errorf("Join %s failed,%s", addr, err.Error())
		if *tryCnt >= th.config.TryJoinTime {
			rst = -1
			return
		}
		//TODO  Bug : th.timer cannot shared by all connect
		th.timer = common.NewTimer(time.Duration(*tryCnt*tryConnectIntervalMs)*time.Millisecond, func() {
			th.timer = nil
			th.tryConnect(addr, tryCnt, rstChan)
		})
		return
	}
	*tryCnt = 0
	ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
	if rsp, err := g.GetClient().JoinRequest(ctx, &inner.JoinReq{
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
		if rsp.Result != 0 {
			logrus.Error("[%s]JoinRequest failed,%s,%s", th.config.NodeId, addr, rsp.Message)
			rst = -3
			return
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
	if !th.store.ExistsNode(info.NodeId) {
		return nil
	}
	return th.members.Add(&Member{
		NodeId:    info.NodeId,
		RaftAddr:  info.RaftAddr,
		InnerAddr: info.InnerAddr,
		Ver:       info.Ver,
		LastIndex: info.LastIndex,
	})
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
			ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
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
			logrus.Debugf("[%s]JoinRequest end,%s", th.config.NodeId, info.NodeId)
			return
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
func (th *MainApp) stopHealthTicker() {
	if th.healthTick != nil {
		th.healthTick.Stop()
		th.healthTick = nil
	}
}
func (th *MainApp) healthTicker() {
	th.healthTick = common.NewTicker(healthIntervalMs*time.Millisecond, func() {
		a := th
		a.members.Foreach(func(member *Member) {
			if member.Con != nil {
				th.Go(func() {
					member.Con.GetRaftClient(func(client inner.RaftClient) {
						if client != nil {
							ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
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
	th.innerLogic.HandleNoHash(func() {
		th.trimJoinFile(th.config.JoinFile)
	})
	if len(th.config.JoinFile) > 0 {
		th.watch.Add(th.config.JoinFile, func(s string) {
			th.innerLogic.HandleNoHash(func() {
				th.trimJoinFile(s)
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
func (th *MainApp) startUpdateTime() {
	if th.updateTimer == nil {
		th.updateTimer = th.NewTimer(time.Duration(updateTimeoutMs)*time.Millisecond, func() {
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
	var cnt int
	oldMem := make([]*Member, 0)
	latestVer := th.latestVersion.Load().(string)
	for _, node := range nodes {
		mem := th.members.Get(string(node.ID))
		if mem == nil {
			logrus.Errorf("[%s]can not found %s", th.config.NodeId, node.ID)
			continue
		}
		if mem.Ver == latestVer {
			cnt++
		} else {
			oldMem = append(oldMem, mem)
		}
	}
	if cnt >= th.config.BootstrapExpect { //达到需求上限，移除旧版本node
		for _, mem := range oldMem {
			if mem.NodeId == th.config.NodeId {
				continue
			}
			if err := th.store.GetRaft().RemoveServer(raft.ServerID(mem.NodeId), 0, 0).Error(); err != nil {
				logrus.Errorf("[%s]RemoveServer err,%s,%s", th.config.NodeId, mem.NodeId, err.Error())
				continue
			}
			logrus.Infof("[%s]RemoveServer successful,%s,%v", th.config.NodeId, mem.NodeId, th.store.GetNodes())
			th.members.LeaveToAll(mem.NodeId)
			th.members.Remove(mem.NodeId)
		}
	}
}
func (th *MainApp) RemoveMember(nodeId string) error {
	if nodeId == th.config.NodeId { //移除自己
		th.DelayStop()
	} else { //移除其他节点
		removed := th.members.Remove(nodeId)
		logrus.Infof("[%s]RemoveMember finished,%s,%v", th.config.NodeId, nodeId, removed)
	}
	return nil
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
