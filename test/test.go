package test

import (
	context "context"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
	"google.golang.org/grpc"
)

type Config struct {
	A int `yaml:"a" help:"help a"`
}

func (Config) Key() string {
	return "test"
}

type testApp struct {
	mainApp *app.MainApp
	config  TestConfig
}

func (th *testApp) Init(app *app.MainApp) error {
	th.mainApp = app
	th.mainApp.OnKeyExpire.Add(func(i interface{}) {
		logrus.Debugf("OnKeyExpire,%v", i)
		if err := th.mainApp.GetStore().Delete(i.(string)).Error(); err != nil {
			logrus.Errorf("OnKeyExpire,remove err,%s", err.Error())
		}
	})
	return nil
}
func (th *testApp) Release() {
}
func (th *testApp) Register(server *grpc.Server) {
	RegisterTestServer(server, &ImplementedTestServer{app: th})
}
func (th *testApp) OnLeader(leader bool) {
}
func (th *testApp) GRpcHandle(f *app.ReplyFuture) {
	th.mainApp.GRpcHandle(f)
}
func (th *testApp) Config() common.FileFlagType {
	return nil
}

type TestConfig struct {
	common.YamlType
	A int    `yaml:"a" json:"a" help:"help testA" default:"3"`
	B string `yaml:"b" json:"b" help:"help testB" default:"d3a"`
}

func (TestConfig) Help() string {
	return "test"
}

func (th *testApp) HandleGetRequest(req *GetReq, rsp *GetRsp, rtv *app.HandlerRtv) {
	//logrus.Debugf("HandleGetRequest,%v", *req)
	//t := time.Now().UnixNano()
	//defer func() {
	//	t = time.Now().UnixNano() - t
	//	if t > 0 {
	//		logrus.Warnf("HandleGetRequest %d", t)
	//	}
	//}()
	rsp.A = req.A
	rsp.B = th.get(req.A)
}
func (th *testApp) HandleSetRequest(req *SetReq, rsp *SetRsp, rtv *app.HandlerRtv) {
	//logrus.Debugf("HandleSetRequest,%v", *req)
	var te common.TimeElapse
	te.Disable()
	te.Call("set_begin")
	rsp.A = "1231"
	for k, v := range req.A {
		rtv.Futures.Add(th.mainApp.GetStore().Set(k, v))
	}
	te.Call("set_end")
}
func (th *testApp) HandleDelRequest(req *DelReq, rsp *DelRsp, rtv *app.HandlerRtv) {
	//logrus.Debugf("HandleDelRequest,%v", *req)
	rsp.A = req.A
	rtv.Futures.Add(th.mainApp.GetStore().Delete(req.A))
}
func (th *testApp) HandleCrashRequest(req *CrashReq, rsp *CrashRsp, rtv *app.HandlerRtv) {
	var p *int
	*p = 0
}
func (th *testApp) HandleLocalRequest(req *LocalReq, rsp *LocalRsp, rtv *app.HandlerRtv) {
	var w sync.WaitGroup
	t := time.Now().UnixNano()
	for i := 0; i < int(req.Cnt); i++ {
		w.Add(1)
		go func(idx int) {
			switch req.Cmd {
			case "set":
				if req.Naked {
					f := th.mainApp.GetStore().Set(strconv.Itoa(idx), idx)
					f.Error()
				} else {
					var req SetReq
					key := strconv.Itoa(idx)
					req.A = map[string]*TestUnit{}
					req.A[key] = &TestUnit{
						A: key,
					}
					f := app.NewReplyFuture(context.WithValue(context.Background(), "hash", key), &req, &SetRsp{TimeLine: []*TimeLineUnit{}})
					th.mainApp.GRpcHandle(f)
					f.Error()
				}
				w.Done()
			case "get":
				if req.Naked {
					th.mainApp.GetStore().Get(strconv.Itoa(idx))
				} else {
					var req GetReq
					key := strconv.Itoa(idx)
					req.A = key
					f := app.NewReplyFuture(context.WithValue(context.Background(), "hash", key), req, &GetRsp{})
					th.mainApp.GRpcHandle(f)
					f.Error()
				}
				w.Done()
			case "del":
				if req.Naked {
					f := th.mainApp.GetStore().Delete(strconv.Itoa(idx))
					f.Error()
				} else {
					var req DelReq
					key := strconv.Itoa(idx)
					req.A = key
					f := app.NewReplyFuture(context.WithValue(context.Background(), "hash", key), req, &DelRsp{})
					th.mainApp.GRpcHandle(f)
					f.Error()
				}
				w.Done()
			}
		}(i)
	}
	w.Wait()
	rsp.Time = time.Now().UnixNano() - t
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

func clearCache() {
	logrus.Debugf("clearCache")
	os.RemoveAll("cache/")
	os.Mkdir("cache/", os.ModePerm)
}
func isTest() bool {
	for _, arg := range os.Args {
		if strings.Contains(arg, "test.v") {
			return true
		}
	}
	return false
}
func init() {
	if isTest() {
		clearCache()
	}
	app.RegisterApp(testApp{})
}
