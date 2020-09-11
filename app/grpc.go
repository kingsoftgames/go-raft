package app

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"

	"google.golang.org/grpc/health"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/store"
	"google.golang.org/grpc"
)

type GRpcClient struct {
	l          sync.Mutex
	client     interface{}
	newClient  interface{}
	con        *grpc.ClientConn
	addr       string
	conTimeout int
}

func NewGRpcClient(conTimeout int, newClient interface{}) *GRpcClient {
	return &GRpcClient{
		conTimeout: conTimeout,
		newClient:  newClient,
	}
}

func (th *GRpcClient) Connect(addr, tag string) error {
	th.l.Lock()
	defer th.l.Unlock()
	if th.client != nil {
		return fmt.Errorf("already connect")
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(th.conTimeout)*time.Millisecond)
	con, err := grpc.DialContext(ctx, addr, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		logrus.Errorf("[%s][GrpcClient]Connect Failed,%s,%s", tag, addr, err.Error())
		return err
	}
	th.client = reflect.ValueOf(th.newClient).Call([]reflect.Value{reflect.ValueOf(con)})[0].Interface()
	th.con = con
	th.addr = addr
	if len(tag) > 0 {
		logrus.Infof("[%s][GrpcClient]Connect Succeed,%s", tag, addr)
	}
	return nil
}
func (th *GRpcClient) Close() {
	th.l.Lock()
	defer th.l.Unlock()
	if th.con != nil {
		_ = th.con.Close()
		th.con = nil
		th.client = nil
	}
}
func (th *GRpcClient) ReConnect(addr string) error {
	logrus.Infof("[GrpcClient]ReConnect %s, %s", th.addr, addr)
	th.Close()
	return th.Connect(addr, "")
}
func (th *GRpcClient) Get() interface{} {
	th.l.Lock()
	defer th.l.Unlock()
	return th.client
}

type innerGRpcClient struct {
	GRpcClient
	Idx int
}

func NewInnerGRpcClient(conTimeout int) *innerGRpcClient {
	c := &innerGRpcClient{}
	c.conTimeout = conTimeout
	c.newClient = inner.NewRaftClient
	return c
}
func (th *innerGRpcClient) GetClient() inner.RaftClient {
	return th.Get().(inner.RaftClient)
}

type GRpcService struct {
	l         sync.RWMutex
	addr      string
	ln        net.Listener
	server    *grpc.Server
	store     *store.RaftStore
	client    *InnerCon
	logicChan chan *ReplyFuture

	mainApp *MainApp

	health *health.Server

	goFunc common.GoFunc
	handle map[string]reflect.Value
}

func New(addr string, conTimeout int, store *store.RaftStore, mainApp *MainApp) *GRpcService {
	return &GRpcService{
		addr:      addr,
		store:     store,
		logicChan: make(chan *ReplyFuture, 2048),
		handle:    map[string]reflect.Value{},
		server:    grpc.NewServer(),
		goFunc:    mainApp,
		mainApp:   mainApp,
	}
}
func (th *GRpcService) Start() error {
	ln, err := net.Listen("tcp", th.addr)
	if err != nil {
		return err
	}
	th.ln = ln
	th.goFunc.Go(func() {
		logrus.Infof("grpc server %s start", th.addr)
		if err := th.server.Serve(th.ln); err != nil {
			logrus.Fatalf("Start failed %s", err.Error())
		}
		logrus.Infof("grpc server %s closed", th.addr)
	})

	//register health
	th.health = health.NewServer()
	//th.health.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	healthgrpc.RegisterHealthServer(th.GetGrpcServer(), th.health)
	//
	//th.store.OnLeaderChg.Add(func(i interface{}) {
	//	if len(i.(string)) == 0 {
	//		return
	//	}
	//	if m := th.mainApp.members.GetByRaftAddr(i.(string)); m != nil {
	//		logrus.Debugf("Leader Chg %s", i.(string))
	//		th.l.Lock()
	//		th.client = m.Con
	//		th.l.Unlock()
	//		if th.client != nil {
	//			th.mainApp.Work()
	//		} else {
	//			logrus.Debugf("[%s]leader not connect Work(%v) ", th.mainApp.config.NodeId, th.mainApp.Check())
	//		}
	//	}
	//
	//})
	return nil
}
func (th *GRpcService) UpdateClient(client *InnerCon) {
	th.l.Lock()
	defer th.l.Unlock()
	th.client = client
}
func (th *GRpcService) SetHealth(health bool) {
	if health {
		th.health.SetServingStatus("", healthgrpc.HealthCheckResponse_SERVING)
	} else {
		th.health.SetServingStatus("", healthgrpc.HealthCheckResponse_NOT_SERVING)
	}
}
func (th *GRpcService) GetGrpcServer() *grpc.Server {
	return th.server
}
func (th *GRpcService) Stop() {
	th.server.GracefulStop()
	th.health.Shutdown()
}

func (th *GRpcService) GetInner() *InnerCon {
	th.l.RLock()
	defer th.l.RUnlock()
	return th.client
}
