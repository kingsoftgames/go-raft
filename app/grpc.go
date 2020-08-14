package app

import (
	"context"
	"fmt"
	"log"
	"net"
	"reflect"
	"sync"
	"time"

	"git.shiyou.kingsoft.com/wangxu13/ppx-app/common"

	"google.golang.org/protobuf/reflect/protoreflect"

	"git.shiyou.kingsoft.com/wangxu13/ppx-app/inner"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/wangxu13/ppx-app/store"
	"github.com/hashicorp/raft"

	"google.golang.org/grpc"
)

type GrpcClient struct {
	l              sync.Mutex
	innerClient    inner.RaftClient
	outerClient    interface{}
	newOuterClient interface{}
	con            *grpc.ClientConn
	addr           string
	timeout        time.Duration
	handler        Handler
}

func (th *GrpcClient) Connect(addr string) error {
	th.l.Lock()
	defer th.l.Unlock()
	if th.innerClient != nil {
		return fmt.Errorf("already connect")
	}
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	con, err := grpc.DialContext(ctx, addr, grpc.WithBlock(), grpc.WithInsecure())
	if err != nil {
		logrus.Errorf("[GrpcClient]Connect Failed,%s,%s", addr, err.Error())
		return err
	}
	th.innerClient = inner.NewRaftClient(con)
	if th.newOuterClient != nil {
		th.outerClient = reflect.ValueOf(th.newOuterClient).Call([]reflect.Value{reflect.ValueOf(con)})[0].Interface()
		th.handler.Register(th.outerClient)
	}
	th.con = con
	th.addr = addr
	logrus.Infof("[GrpcClient]Connect Succeed,%s", addr)
	return nil
}
func (th *GrpcClient) Stop() {
	th.l.Lock()
	defer th.l.Unlock()
	if th.con != nil {
		_ = th.con.Close()
		th.con = nil
		th.innerClient = nil
		th.outerClient = nil
	}
}
func (th *GrpcClient) ReConnect(addr string) error {
	logrus.Infof("[GrpcClient]ReConnect %s, %s", th.addr, addr)
	if addr == th.addr {
		logrus.Infof("Reconnect same addr %s\n", addr)
		return nil
	}
	th.Stop()
	return th.Connect(addr)
}
func (th *GrpcClient) GetInner() inner.RaftClient {
	th.l.Lock()
	defer th.l.Unlock()
	return th.innerClient
}
func (th *GrpcClient) GetOuter() interface{} {
	th.l.Lock()
	defer th.l.Unlock()
	return th.outerClient
}
func (th *GrpcClient) InitNewOutFunction(newGrpcClientFunction interface{}) {
	th.newOuterClient = newGrpcClientFunction
}
func (th *GrpcClient) outCall(method string, values []reflect.Value) ([]reflect.Value, error) {
	return th.handler.Handle(method, values)
}

type Service struct {
	addr      string
	ln        net.Listener
	server    *grpc.Server
	store     *store.RaftStore
	client    *GrpcClient
	logicChan chan *ReplyFuture

	handle map[string]reflect.Value
}

func New(addr string, store *store.RaftStore) *Service {
	return &Service{
		addr:      addr,
		store:     store,
		client:    &GrpcClient{},
		logicChan: make(chan *ReplyFuture, 2048),
		handle:    map[string]reflect.Value{},
		server:    grpc.NewServer(),
	}
}
func (th *Service) Start() error {
	ln, err := net.Listen("tcp", th.addr)
	if err != nil {
		return err
	}
	th.ln = ln
	go func() {
		if err := th.server.Serve(th.ln); err != nil {
			log.Fatalf("Start failed %s", err.Error())
		}
	}()
	th.store.OnStateChg.Add(func(i interface{}) {
		switch i.(raft.RaftState) {
		case raft.Leader:
		}
	})
	th.store.OnLeaderChg.Add(func(i interface{}) {
		if i.(string) == "" {
			return
		}
		if apiAddr, err := th.store.GetApiAddr(i.(string)); err == nil {
			_ = th.client.ReConnect(apiAddr)
		} else {
			logrus.Infof("GetApiAddr error ,%s", err.Error())
		}

	})
	return nil
}
func (th *Service) GetGrpcServer() *grpc.Server {
	return th.server
}
func (th *Service) Stop() {
	th.server.GracefulStop()
}
func (th *Service) IsLeader() bool {
	return th.store.IsLeader()
}
func (th *Service) IsFollower() bool {
	return th.store.IsFollower()
}

func (th *Service) GetInner() inner.RaftClient {
	if th.client.GetInner() == nil {
		addr, _ := th.store.GetApiAddr(string(th.store.GetRaft().Leader()))
		if err := th.client.ReConnect(addr); err != nil {
			return nil
		}
	}
	return th.client.GetInner()
}
func (th *Service) GetOuter() interface{} {
	if th.client.GetOuter() == nil {
		addr, _ := th.store.GetApiAddr(string(th.store.GetRaft().Leader()))
		if err := th.client.ReConnect(addr); err != nil {
			return nil
		}
	}
	return th.client.GetOuter()
}

func (th *Service) SetNewOutClient(f interface{}) {
	th.client.InitNewOutFunction(f)
}
func (th *Service) OutCall(req protoreflect.ProtoMessage) ([]reflect.Value, error) {
	if outer := th.GetOuter(); outer != nil {
		return th.client.outCall(common.GetHandleFunctionName(req), []reflect.Value{reflect.ValueOf(outer), reflect.ValueOf(context.Background()), reflect.ValueOf(req)})
	}
	//丢失链接，可能是leader挂了
	return nil, fmt.Errorf("lost OuterConnect")
}
