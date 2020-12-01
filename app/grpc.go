package app

import (
	"context"
	"fmt"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
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
	conTimeout time.Duration
}

func NewGRpcClient(conTimeout time.Duration, newClient interface{}) *GRpcClient {
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
	ctx, _ := context.WithTimeout(context.Background(), th.conTimeout)
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

func NewInnerGRpcClient(conTimeout time.Duration) *innerGRpcClient {
	c := &innerGRpcClient{}
	c.conTimeout = conTimeout
	c.newClient = inner.NewRaftClient
	return c
}
func (th *innerGRpcClient) GetClient() inner.RaftClient {
	return th.Get().(inner.RaftClient)
}

type GRpcService struct {
	tag        string
	addr       string
	ln         net.Listener
	server     *grpc.Server
	store      *store.RaftStore
	client     atomic.Value
	lastClient atomic.Value
	logicChan  chan *ReplyFuture

	mainApp *MainApp

	health *health.Server

	goFunc common.GoFunc
	handle map[string]reflect.Value
}

func New(addr string, store *store.RaftStore, mainApp *MainApp, tag string) *GRpcService {
	return &GRpcService{
		tag:       tag,
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
	//register health
	th.health = health.NewServer()
	healthgrpc.RegisterHealthServer(th.GetGrpcServer(), th.health)

	th.goFunc.Go(func() {
		logrus.Infof("[%s.%s]grpc server %s start", th.mainApp.getNodeName(), th.tag, th.addr)
		if err := th.server.Serve(th.ln); err != nil {
			logrus.Fatalf("[%s.%s]Start failed %s", th.mainApp.getNodeName(), th.tag, err.Error())
		}
		logrus.Infof("[%s.%s]grpc server %s closed", th.mainApp.getNodeName(), th.tag, th.addr)
	})
	return nil
}
func (th *GRpcService) UpdateClient(client *InnerCon) {
	if l := th.client.Load(); l != nil {
		th.lastClient.Store(l)
	}
	th.client.Store(client)
	if client != nil {
		logrus.Debugf("[%s.%s]UpdateClient,%s", th.mainApp.getNodeName(), th.tag, client.addr)
	} else {
		logrus.Debugf("[%s.%s]UpdateClient,%s", th.mainApp.getNodeName(), th.tag, "")
	}
}
func (th *GRpcService) SetHealth(health bool) {
	logrus.Infof("[%s.%s]SetHealth,%v", th.mainApp.getNodeName(), th.tag, health)
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
	logrus.Infof("[%s.%s]GRpcService.Stop,%s", th.mainApp.getNodeName(), th.tag, th.addr)
	defer logrus.Infof("[%s.%s]GRpcService.Stop finished,%s", th.mainApp.getNodeName(), th.tag, th.addr)
	th.server.GracefulStop()
	th.health.Shutdown()
}

func (th *GRpcService) GetInner() *InnerCon {
	t := time.Now().UnixNano()
	defer func() {
		if dif := time.Now().UnixNano() - t; dif > 1e9 {
			logrus.Errorf("[%s]GetInner[%v]", th.mainApp.getNodeName(), dif)
		}
	}()
	if c := th.client.Load(); c != nil {
		return c.(*InnerCon)
	}
	return nil
}
func (th *GRpcService) GetLastInner() *InnerCon {
	if c := th.lastClient.Load(); c != nil {
		return c.(*InnerCon)
	}
	return nil
}
