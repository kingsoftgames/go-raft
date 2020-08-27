package app

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
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
	healthIntervalMs     = 1000
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
	service *Service
	config  *common.Configure

	store *store.RaftStore

	runLogic common.LogicChan

	handler Handler

	http *httpApi

	tryCnt int

	OnLeaderChg common.SafeEvent

	stopWait sync.WaitGroup

	members    *MemberList
	healthTick *common.Ticker
}

func NewMainApp(app IApp, exitWait *common.GracefulExit) *MainApp {
	mainApp := &MainApp{
		app:     app,
		config:  common.NewDefaultConfigure(),
		members: &MemberList{},
	}
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
	th.runLogic.Init(runtime.NumCPU()-1, th)
	th.config.StoreDir = filepath.Join(th.config.StoreDir, th.config.NodeId)
	th.store = store.New(th.config, nil, th)
	th.store.OnLeader.Add(func(i interface{}) {
		th.app.OnLeader(i.(bool))
	})
	th.store.OnLeaderChg.Add(func(i interface{}) {
		th.OnLeaderChg.Emit(i)
	})
	th.members.OnConEvent.Add(func(i interface{}) {
		leaderAddr := string(th.store.GetRaft().Leader())
		logrus.Debugf("OnConEvent %s , %s", i.(*Member).RaftAddr, leaderAddr)
		if i.(*Member).RaftAddr == string(th.store.GetRaft().Leader()) {
			th.Work()
			logrus.Debugf("[%s]connect Work(%v)", th.config.NodeId, th.Check())
		}
	})
	th.store.OnStateChg.Add(func(i interface{}) {
		switch i.(raft.RaftState) {
		case raft.Leader, raft.Follower:
			th.Work()
		default:
			th.Idle()
		}
	})
	if err := th.store.Open(th.config.LogConfig.Level, common.NewFileLog(th.config.LogConfig, "raft")); err != nil {
		logrus.Errorf("store open err,%s", err.Error())
		return -3
	}
	th.service = New(th.config.GrpcApiAddr, th.config.ConnectTimeoutMs, th.store, th)
	inner.RegisterRaftServer(th.service.GetGrpcServer(), &RaftServerGRpc{App: th})
	th.app.Register(th.service.GetGrpcServer())
	if err := th.service.Start(); err != nil {
		logrus.Errorf("service start err,%s", err.Error())
		return -4
	}
	th.WaitGo()
	if rst := th.tryJoin(); rst != 0 {
		return -5
	}
	th.handler.Register(th.app)
	th.initHttpApi()
	th.joinSelf()
	th.healthTicker()
	logrus.Infof("[%s]Init finished", th.config.NodeId)
	return 0
}
func (th *MainApp) release() {
	defer func() {
		th.Done()
		th.stopWait.Done()
	}()
	th.app.Release()
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
	if th.service.IsLeader() {
		leader()
	} else if th.service.IsFollower() {
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
			client := th.service.GetInner()
			if client == nil {
				f.response(fmt.Errorf("node can not work"))
				return
			}
			rsp, err := th.service.GetInner().TransGrpcRequest(ctx, req)
			if err != nil {
				f.response(err)
				return
			}
			f.response(common.Decode(rsp.Data, f.rsp))
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
		defer func() {
			th.release()
		}()
		th.runLogic.Start()
	})
}

func (th *MainApp) Stop() {
	//first stop runLogic , make sure all running logic deal over
	th.runLogic.Stop()
	th.store.Shutdown()
	th.service.Stop()
	th.Idle()
	if th.http != nil {
		th.http.close()
	}
	if th.healthTick != nil {
		th.healthTick.Stop()
		th.healthTick = nil
	}
}
func (th *MainApp) GetStore() *store.RaftStore {
	return th.store
}

func (th *MainApp) initHttpApi() {
	th.http = newHttpApi(th)
	th.http.init(th.config.HttpApiAddr)
}
func (th *MainApp) tryConnect(addr string, rstChan chan int) {
	th.tryCnt++
	logrus.Infof("tryConnect %s,%d/%d", addr, th.tryCnt, th.config.TryJoinTime)
	g := NewInnerGRpcClient(th.config.ConnectTimeoutMs)
	if err := g.Connect(addr); err != nil {
		logrus.Errorf("Join %s failed,%s", addr, err.Error())
		if th.tryCnt >= th.config.TryJoinTime {
			rstChan <- -1
			return
		}
		common.NewTimer(time.Duration(th.tryCnt*tryConnectIntervalMs)*time.Millisecond, func() {
			th.tryConnect(addr, rstChan)
		})
		return
	}
	ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
	if rsp, err := g.GetClient().JoinRequest(ctx, &inner.JoinReq{
		Addr:    th.config.RaftAddr,
		ApiAddr: th.config.GrpcApiAddr,
		NodeId:  th.config.NodeId,
	}); err != nil {
		logrus.Errorf("[%s]JoinRequest error,%s", th.config.NodeId, err.Error())
		rstChan <- -2
		return
	} else {
		if rsp.Result != 0 {
			logrus.Error("[%s]JoinRequest result error,%s", th.config.NodeId, rsp.Message)
			rstChan <- -3
			return
		}
	}
	rstChan <- 0
}
func (th *MainApp) tryJoin() (rst int) {
	if len(th.config.JoinAddr) == 0 {
		return 0
	}
	//如果作为join启动，那么当前node的BootstrapExpect设置为0
	th.config.BootstrapExpect = 0
	addrs := strings.Split(th.config.JoinAddr, ",")
	rstChan := make(chan int)
	defer close(rstChan)
	for _, addr := range addrs {
		th.Go(func() {
			th.tryConnect(addr, rstChan)
		})
		rst = <-rstChan
		if rst == 0 {
			break
		}
	}
	return
}
func (th *MainApp) joinSelf() {
	th.members.selfNodeId = th.config.NodeId
	th.Join(th.config.NodeId, th.config.RaftAddr, th.config.GrpcApiAddr)
}
func (th *MainApp) OnlyJoin(nodeId string, addr string, apiAddr string) error {
	return th.members.Add(&Member{
		NodeId:   nodeId,
		RaftAddr: addr,
		GrpcAddr: apiAddr,
	})
}
func (th *MainApp) Join(nodeId string, addr string, apiAddr string) error {
	if th.members.Get(nodeId) != nil {
		th.members.SynMemberToAll(th.config.Bootstrap, th.config.BootstrapExpect)
		return nil
	}
	if err := th.members.Add(&Member{
		NodeId:   nodeId,
		RaftAddr: addr,
		GrpcAddr: apiAddr,
	}); err != nil {
		return err
	}
	if th.config.NodeId == nodeId { // join self
		return nil
	}
	//同步下
	th.members.SynMemberToAll(th.config.Bootstrap, th.config.BootstrapExpect)
	if th.GetStore().IsLeader() {
		if err := th.GetStore().Join(nodeId, addr, apiAddr); err != nil {
			return err
		}
	} else {
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
func (th *MainApp) healthTicker() {
	th.healthTick = common.NewTicker(healthIntervalMs*time.Millisecond, func() {
		th.members.Foreach(func(member *Member) {
			if member.Con != nil {
				ctx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
				_, err := member.Con.GetClient().HealthRequest(ctx, &inner.HealthReq{
					NodeId:  th.config.NodeId,
					Addr:    th.config.RaftAddr,
					ApiAddr: th.config.GrpcApiAddr,
				})
				member.Health(err == nil)
			}
		})
	})
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
