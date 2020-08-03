package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/raft"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/app"
	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/common"
	"google.golang.org/grpc"
)

//bench for app

type testApp struct {
	mainApp *app.MainApp
	app.BaseHttpHandler
}

func (th *testApp) Init(app *app.MainApp) error {
	th.mainApp = app
	return nil
}
func (th *testApp) Release() {
}
func (th *testApp) Register(server *grpc.Server) (newGrpcClientFunction interface{}) {
	RegisterTestServer(server, &ImplementedTestServer{app: th})
	return NewTestClient
}
func (th *testApp) OnLeader(leader bool) {
}
func (th *testApp) GRpcHandle(f *app.ReplyFuture) {
	th.mainApp.GRpcHandle(f)
}
func (th *testApp) OnHttpRegister() {
	th.Put("/api/test/get", th.HandleGetRequest)
	th.Put("/api/test/set", th.HandleSetRequest)
	th.Put("/api/test/del", th.HandleDelRequest)
}

func (th *testApp) HandleGetRequest(req *GetReq, rsp *GetRsp, rtv *app.HandlerRtv) {
	rsp.A = req.A
	rsp.B = th.get(req.A)
}
func (th *testApp) HandleSetRequest(req *SetReq, rsp *SetRsp, rtv *app.HandlerRtv) {
	rsp.A = "1231"
	for k, v := range req.A {
		rtv.Futures.Add(th.mainApp.GetStore().Set(k, v))
	}
}
func (th *testApp) HandleDelRequest(req *DelReq, rsp *DelRsp, rtv *app.HandlerRtv) {
	rsp.A = req.A
	rtv.Futures.Add(th.mainApp.GetStore().Delete(req.A))
}
func (th *testApp) get(id string) *TestUnit {
	if v, err := th.mainApp.GetStore().Get(id); err != nil || v == nil {
		return nil
	} else {
		var team TestUnit
		if err = common.Decode(v.([]byte), &team); err != nil {
			return nil
		}
		return &team
	}
}
func init() {
	app.RegisterApp(testApp{})
}

var count = 5000
var timeout = 5000 * time.Millisecond

func singleApp(t *testing.T, clientFunc func()) {
	var exitWait sync.WaitGroup
	app := app.NewMainApp(app.CreateApp("test"), &exitWait)
	app.Init("test_leader.yaml")
	app.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			go func() {
				n := time.Now().UnixNano()
				clientFunc()
				t.Logf("singleApp %f ms", float64(time.Now().UnixNano()-n)/10e6)
				app.Stop()
			}()
		}
	})
	t.Run("Start", func(t *testing.T) {
		app.Start()
		t.Parallel()
	})
	exitWait.Wait()
}
func Test_markSingleAppGRpc(t *testing.T) {
	singleApp(t, func() {
		c := newClient("127.0.0.1:8310")
		var w sync.WaitGroup

		for i := 0; i < count; i++ {
			w.Add(1)
			go func() {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c, "set", key, 1, true)
				runClient(t, c, "get", key, 1, true)
				runClient(t, c, "del", key, 1, true)
				runClient(t, c, "get", key, 1, true)
			}()
		}
		w.Wait()
	})
}
func Test_markSingleAppHttp(t *testing.T) {
	singleApp(t, func() {
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func() {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, "http://127.0.0.1:8320", "/api/test/set", key, 1, true)
				httpClient(t, "http://127.0.0.1:8320", "/api/test/get", key, 1, true)
				httpClient(t, "http://127.0.0.1:8320", "/api/test/del", key, 1, true)
				httpClient(t, "http://127.0.0.1:8320", "/api/test/get", key, 1, true)
			}()
		}
		w.Wait()
	})
}
func clusterApp(t *testing.T, clientFunc func()) {
	var exitWait sync.WaitGroup
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	appLeader.Init("test_cluster_leader.yaml")
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			appFollower := app.NewMainApp(app.CreateApp("test"), &exitWait)
			appFollower.OnLeaderChg.Add(func(i interface{}) {
				go func() {
					n := time.Now().UnixNano()
					clientFunc()
					t.Logf("clusterApp %f ms", float64(time.Now().UnixNano()-n)/10e6)
					appLeader.Stop()
					appFollower.Stop()
				}()
			})
			appFollower.Init("test_follower.yaml")
			go appFollower.Start()
		}
	})
	go appLeader.Start()
	exitWait.Wait()
}

func Test_ClusterAppGRpc(t *testing.T) {
	clusterApp(t, func() {
		c := []*app.GrpcClient{newClient("127.0.0.1:8311"), newClient("127.0.0.1:8312")}
		var w sync.WaitGroup

		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				//runClient(t, c[idx%len(c)], "get", key, 1, true)
				//runClient(t, c[idx%len(c)], "get", key, 1, true)
				runClient(t, c[idx%len(c)], "get", key, 1, false)
				runClient(t, c[idx%len(c)], "get", key, 1, false)

				runClient(t, c[idx%len(c)], "set", key, 1, false)
				runClient(t, c[idx%len(c)], "set", key, 1, false)
				//runClient(t, c[idx%len(c)], "set", key, 1, false)
				//runClient(t, c[idx%len(c)], "set", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func Test_ClusterAppHttp(t *testing.T) {
	clusterApp(t, func() {
		addr := []string{"http://127.0.0.1:8321", "http://127.0.0.1:8322"}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, true)

				//httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, true)
				//httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, true)
				//httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, true)
				//httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, true)


				//httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, false)
				//httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, false)
				//httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, false)
				//httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func clusterApp2(t *testing.T, clientFunc func()) {
	var exitWait sync.WaitGroup
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	appLeader.Init("test_cluster_leader.yaml")
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			appFollower := app.NewMainApp(app.CreateApp("test"), &exitWait)
			appFollower.OnLeaderChg.Add(func(i interface{}) {
				appFollower2 := app.NewMainApp(app.CreateApp("test"), &exitWait)
				appFollower2.OnLeaderChg.Add(func(i interface{}) {
					go func() {
						n := time.Now().UnixNano()
						clientFunc()
						t.Logf("clusterApp2 %f ms", float64(time.Now().UnixNano()-n)/10e6)
						appLeader.Stop()
						appFollower.Stop()
						appFollower2.Stop()
					}()
				})
				appFollower2.Init("test_follower2.yaml")
				go appFollower2.Start()
			})
			appFollower.Init("test_follower.yaml")
			go appFollower.Start()
		}
	})
	go appLeader.Start()
	exitWait.Wait()
}

func Test_ClusterApp2GRpc(t *testing.T) {
	clusterApp(t, func() {
		c := []*app.GrpcClient{newClient("127.0.0.1:8311"), newClient("127.0.0.1:8312"), newClient("127.0.0.1:8313")}
		var w sync.WaitGroup

		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c[idx%len(c)], "set", key, 1, false)
				runClient(t, c[idx%len(c)], "get", key, 1, false)
				runClient(t, c[idx%len(c)], "del", key, 1, false)
				runClient(t, c[idx%len(c)], "get", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func Test_ClusterApp2Http(t *testing.T) {
	clusterApp2(t, func() {
		addr := []string{"http://127.0.0.1:8321", "http://127.0.0.1:8322", "http://127.0.0.1:8323"}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, addr[idx%len(addr)], "/api/test/set", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/api/test/del", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/api/test/get", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func newClient(addr string) *app.GrpcClient {
	c := &app.GrpcClient{}
	c.InitNewOutFunction(NewTestClient)
	if err := c.Connect(addr); err != nil {
		log.Fatalf("connect failed,%s", err.Error())
	}
	return c
}
func newTestUnit() *TestUnit {
	return &TestUnit{
		A: "123",
		B: 1,
		C: 2,
		D: "321",
	}
}
func httpPost(hash string, url, contentType string, body io.Reader) (resp *http.Response, err error) {
	c := http.DefaultClient
	ctx := context.WithValue(context.Background(), "hash", hash)
	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	if len(hash) > 0 {
		req.Header.Set("hash", hash)
	}
	return c.Do(req)
}
func httpClient(t *testing.T, addr, path string, key string, writeTimes int, hash bool) {
	var req interface{}
	switch path {
	case "/api/test/get":
		req = &GetReq{
			A: key,
		}
	case "/api/test/set":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		req = &SetReq{
			A: A,
		}
	case "/api/test/del":
		req = &DelReq{
			A: key,
		}
	}
	b, _ := json.Marshal(req)
	//rsp, err := http.DefaultClient.Post(addr+path, "application/json", bytes.NewReader(b))
	if !hash {
		key = ""
	}
	rsp, err := httpPost(key, addr+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Logf("http rsp err ,%s", err.Error())
	} else {
		bb := make([]byte, 4096)
		rsp.Body.Read(bb)
		//t.Logf("http rsp : %s", string(bb))
	}
}
func runClient(t *testing.T, c *app.GrpcClient, cmd string, key string, writeTimes int, hash bool) {
	header := &Header{}
	if hash {
		header.Hash = key
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	switch cmd {
	case "get":
		_, e := c.GetOuter().(TestClient).GetRequest(ctx, &GetReq{
			Header: header,
			A:      key,
			B:      0,
			C:      0,
		})
		if e != nil {
			t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("getrsp, %v", *rsp)
		}
	case "set":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		_, e := c.GetOuter().(TestClient).SetRequest(ctx, &SetReq{
			Header: header,
			A:      A,
		})
		if e != nil {
			t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("setrsp, %v", *rsp)
		}
	case "del":
		_, e := c.GetOuter().(TestClient).DelRequest(ctx, &DelReq{
			Header: header,
			A:      key,
		})
		if e != nil {
			t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("delrsp, %v", *rsp)
		}
	default:
	}
}
