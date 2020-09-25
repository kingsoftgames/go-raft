package test

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
	"google.golang.org/grpc"
)

type testApp struct {
	mainApp *app.MainApp
}

func (th *testApp) Init(app *app.MainApp) error {
	th.mainApp = app
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

func (th *testApp) HandleGetRequest(req *GetReq, rsp *GetRsp, rtv *app.HandlerRtv) {
	//logrus.Debugf("HandleGetRequest,%v", *req)
	rsp.A = req.A
	rsp.B = th.get(req.A)
}
func (th *testApp) HandleSetRequest(req *SetReq, rsp *SetRsp, rtv *app.HandlerRtv) {
	//logrus.Debugf("HandleSetRequest,%v", *req)
	rsp.A = "1231"
	for k, v := range req.A {
		rtv.Futures.Add(th.mainApp.GetStore().Set(k, v))
	}
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
