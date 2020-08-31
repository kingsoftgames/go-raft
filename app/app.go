package app

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
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
	healthIntervalMs     = 2000
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
}

func NewMainApp(app IApp, exitWait *common.GracefulExit) *MainApp {
	mainApp := &MainApp{
		app:     app,
		config:  common.NewDefaultConfigure(),
		members: &MemberList{},
	}
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
	th.runLogic.Init(runtime.NumCPU()-1, th)
	th.innerLogic.Init(1, th)
	th.config.StoreDir = filepath.Join(th.config.StoreDir, th.config.NodeId)
	th.store = store.New(th.config, nil, th)
	th.store.OnLeader.Add(func(i interface{}) {
		th.app.OnLeader(i.(bool))
	})
	th.store.OnLeaderChg.Add(func(i interface{}) {
		th.OnLeaderChg.Emit(i)
	})
	th.members.OnConEvent.Add(func(i interface{}) {
		//leaderAddr := string(th.store.GetRaft().Leader())
		//logrus.Debugf("OnConEvent %s , %s", i.(*Member).RaftAddr, leaderAddr)
		//if i.(*Member).RaftAddr == string(th.store.GetRaft().Leader()) {
		//	th.Work()
		//	logrus.Debugf("[%s]connect Work(%v)", th.config.NodeId, th.Check())
		//}
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
	th.joinSelf()
	th.service = New(th.config.GrpcApiAddr, th.config.ConnectTimeoutMs, th.store, th)
	inner.RegisterRaftServer(th.service.GetGrpcServer(), &RaftServerGRpc{App: th})
	th.app.Register(th.service.GetGrpcServer())
	if err := th.service.Start(); err != nil {
		logrus.Errorf("service start err,%s", err.Error())
		return -4
	}
	th.WaitGo()
	if rst := th.tryJoin(th.config.JoinAddr, true); rst != 0 {
		return -5
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
			con := th.service.GetInner()
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

func (th *MainApp) Stop() {
	logrus.Infof("[%s]Stop,begin", th.config.NodeId)
	//first stop runLogic , make sure all running logic deal over
	if th.timer != nil {
		th.timer.Stop()
		th.timer = nil
	}
	th.watch.Stop()
	th.runLogic.Stop()
	th.innerLogic.Stop()
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
	if err := g.Connect(addr); err != nil {
		logrus.Errorf("Join %s failed,%s", addr, err.Error())
		if *tryCnt >= th.config.TryJoinTime {
			rst = -1
			return
		}
		th.timer = common.NewTimer(time.Duration(*tryCnt*tryConnectIntervalMs)*time.Millisecond, func() {
			th.timer = nil
			th.tryConnect(addr, tryCnt, rstChan)
		})
		return
	}
	*tryCnt = 0
	ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
	if rsp, err := g.GetClient().JoinRequest(ctx, &inner.JoinReq{
		Addr:    th.config.RaftAddr,
		ApiAddr: th.config.GrpcApiAddr,
		NodeId:  th.config.NodeId,
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
				th.Go(func() {
					member.Con.GetRaftClient(func(client inner.RaftClient) {
						if client != nil {
							ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
							rsp, err := client.HealthRequest(ctx, &inner.HealthReq{
								NodeId:   th.config.NodeId,
								Addr:     th.config.RaftAddr,
								ApiAddr:  th.config.GrpcApiAddr,
								SendTime: time.Now().UnixNano(),
							})
							if err != nil {
								logrus.Warnf("[%s]healthTicker(failed) to [%s], %s", th.config.NodeId, member.NodeId, err.Error())
							} else {
								logrus.Debugf("[%s]healthTicker(ok) to [%s], %d", th.config.NodeId, member.NodeId, (time.Now().UnixNano()-rsp.SendTime)/1e6)
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
			th.tryJoin(addrs, false)
		}
	}
}
func (th *MainApp) watchJoinFile() {
	th.trimJoinFile(th.config.JoinFile)
	if len(th.config.JoinFile) > 0 {
		th.watch.Add(th.config.JoinFile, func(s string) {
			th.innerLogic.HandleNoHash(func() {
				th.trimJoinFile(s)
			})
		})
	}
	th.watch.Start()
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
