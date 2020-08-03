package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"sync"
	"time"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/inner"

	"google.golang.org/grpc"

	"google.golang.org/protobuf/reflect/protoreflect"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/common"
	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/store"
	_ "github.com/gin-gonic/gin"
	_ "github.com/go-yaml/yaml"
	"github.com/sirupsen/logrus"
)

type IGRpcHandler interface {
	GRpcHandle(*ReplyFuture)
}
type IApp interface {
	IHttpHandler
	IGRpcHandler
	Init(app *MainApp) error
	Release()
	Register(server *grpc.Server) (newGrpcClientFunction interface{})
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

	service *Service
	config  *configure

	exitChan chan os.Signal

	store *store.RaftStore

	//runChan common.RunChanType

	runLogic common.LogicChan

	handler Handler

	exitWait *sync.WaitGroup

	http *httpApi

	qps       int64
	totalTime int64

	OnLeaderChg common.SafeEvent
}

func NewMainApp(app IApp, exitWait *sync.WaitGroup) *MainApp {
	exitWait.Add(1)
	return &MainApp{
		app:      app,
		config:   newDefaultConfigure(),
		exitChan: make(chan os.Signal, 1),
		exitWait: exitWait,
		//runChan:  make(common.RunChanType, 4096),
	}
}

func (th *MainApp) Init(configPath string) int {
	th.config = initConfigureFromFile(configPath)
	common.InitLog()
	common.InitCodec(th.config.Codec)
	if err := th.app.Init(th); err != nil {
		logrus.Errorf("app Init err,%s", err.Error())
		return -2
	}
	th.runLogic.Init(runtime.NumCPU() - 1)
	th.config.StoreDir = th.config.StoreDir + th.config.NodeId
	th.store = store.New(th.config.StoreInMem, th.config.StoreDir, th.config.RaftAddr, th.config.GrpcApiAddr, th.runLogic.Get())
	th.store.OnLeader.Add(func(i interface{}) {
		th.app.OnLeader(i.(bool))
	})
	th.store.OnLeaderChg.Add(func(i interface{}) {
		th.OnLeaderChg.Emit(i)
	})
	if err := th.store.Open(th.config.JoinAddr, th.config.NodeId); err != nil {
		logrus.Errorf("store open err,%s", err.Error())
		return -3
	}
	if len(th.config.JoinAddr) != 0 { //不是作为leader启动，join
		var g GrpcClient
		if err := g.Connect(th.config.JoinAddr); err != nil {
			logrus.Errorf("Join %s failed,%s", th.config.JoinAddr, err.Error())
		}
		ctx, _ := context.WithTimeout(context.Background(), time.Duration(th.config.ConnectTimeoutMs)*time.Millisecond)
		if rsp, err := g.GetInner().JoinRequest(ctx, &inner.JoinReq{
			Addr:    th.config.RaftAddr,
			ApiAddr: th.config.GrpcApiAddr,
			NodeId:  th.config.NodeId,
		}); err != nil {
			logrus.Errorf("JoinRequest error,%s", err.Error())
			return -4
		} else {
			if rsp.Result != 0 {
				logrus.Error("JoinRequest failed,%s", rsp.Message)
				return -4
			}
		}
	}

	th.service = New(th.config.GrpcApiAddr, th.store)
	inner.RegisterRaftServer(th.service.GetGrpcServer(), &RaftServerGrpc{App: th})
	newGrpcClientFunction := th.app.Register(th.service.GetGrpcServer())

	if err := th.service.Start(); err != nil {
		logrus.Errorf("service start err,%s", err.Error())
		return -5
	}

	th.handler.Register(th.app)
	th.service.SetNewOutClient(newGrpcClientFunction)

	signal.Notify(th.exitChan, os.Interrupt)
	signal.Notify(th.exitChan, os.Kill)
	th.initHttpApi(th.app)

	return 0
}
func (th *MainApp) release() {
	defer th.exitWait.Done()
	th.app.Release()
	th.store.Release()
	if th.totalTime > 0 && th.qps > 0 {
		logrus.Infof("call %d,time,%d,per %d nans,qps=%d", th.qps, th.totalTime, th.totalTime/th.qps, 1e9/(th.totalTime/th.qps))
	}
}

func (th *MainApp) GRpcHandle(f *ReplyFuture) {
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
		handleContext(&th.runLogic, f.ctx, h)
		//if th.runChan == nil { //没有主协程
		//	go h()
		//} else {
		//	th.runChan <- h
		//}
	}
	if th.service.IsLeader() {
		leader()
	} else if th.service.IsFollower() {
		if readOnly := f.ctx.Value("readOnly"); readOnly != nil && readOnly.(bool) {
			leader()
			return
		}
		//转发给leader
		go func() {
			rsp, err := th.service.OutCall(f.req.(protoreflect.ProtoMessage))
			if err != nil {
				f.response(err)
				return
			}
			if rsp[1].IsValid() && rsp[1].Interface() != nil {
				f.response(rsp[1].Interface().(error))
			} else {
				f.rsp = rsp[0].Interface()
				f.response(nil)
			}
		}()
	} else {
		f.response(fmt.Errorf("invalid node Candidate"))
	}
}
func (th *MainApp) HttpCall(ctx context.Context, path string, data []byte) ([]byte, error) {
	return th.http.call(ctx, path, data)
}
func (th *MainApp) Start() {
	defer func() {
		th.release()
	}()
	th.runLogic.Start()
	//
	//for {
	//	select {
	//	case f := <-th.runChan:
	//		n := time.Now().UnixNano()
	//		f()
	//		th.totalTime = th.totalTime + (time.Now().UnixNano() - n)
	//		th.qps++
	//	case <-th.exitChan:
	//		return
	//	}
	//}
}

func (th *MainApp) Stop() {
	if th.exitChan != nil {
		th.exitChan <- os.Kill
	}
	th.runLogic.Stop()
}
func (th *MainApp) GetStore() *store.RaftStore {
	return th.store
}

func (th *MainApp) initHttpApi(httpHandler IHttpHandler) {
	th.http = newHttpApi(th, httpHandler)
	th.http.init(th.config.HttpApiAddr)
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
