package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/raft"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
)

var count = 100
var timeout = 10 * time.Second

func Test_markSingleAppGRpc(t *testing.T) {
	t.Log(common.GetStringSum(""))
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *app.Configure) {
	})
	singleAppTemplate(t, func() {
		time.Sleep(100 * time.Second)

		c := newClient("127.0.0.1:18310")
		var w sync.WaitGroup
		var te common.TimeElapse
		te.Call("begin")
		//app.DebugTraceFutureLine = true
		for i := 0; i < count; i++ {
			w.Add(1)
			go func() {
				key := strconv.Itoa(rand.Int())
				runClient(t, c, "set", key, 1, true)
				//runClient(t, c, "get", key, 1, true)
				//runClient(t, c, "del", key, 1, true)
				//runClient(t, c, "get", key, 1, true)
				w.Done()
			}()
		}
		w.Wait()
		time.Sleep(10 * time.Second)
		te.Call("end")
		printTotalTimeLine()
	})
}

func Test_markSingleAppGRpcLocalTest(t *testing.T) {
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *app.Configure) {
	})
	singleAppTemplate(t, func() {
		GRpcLocalQuery("127.0.0.1:18310", "set", count, false, timeout)
	})
}
func Test_markClusterAppGRpcLocalTest(t *testing.T) {
	genTestClusterApp2GrpcYaml()
	clusterApp2Template(t, func() {
		GRpcLocalQuery("127.0.0.1:18310", "set", count, false, timeout)
	})
}

func Test_RaftSingle_Apply(t *testing.T) {
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *app.Configure) {
	})
	var exitWait common.GracefulExit
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	if err := appLeader.Init(leaderYaml); err != nil {
		t.Errorf("appLeader Init error,%s", err.Error())
		return
	}
	clientFunc := func() {
		var w sync.WaitGroup
		var te common.TimeElapse
		te.Call("begin")
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				//var req SetReq
				//key := strconv.Itoa(idx)
				//req.A = map[string]*TestUnit{}
				//req.A[key] = &TestUnit{
				//	A: key,
				//}
				//f := app.NewReplyFuture(context.WithValue(context.Background(), "hash", key), &req, &SetRsp{TimeLine: []*TimeLineUnit{}})
				//appLeader.GRpcHandle(f)
				f := appLeader.GetStore().Set(strconv.Itoa(idx), idx)
				f.Error()
				w.Done()
				//runChan.Handle(idx, 10*time.Second, func(err error) {
				//	f := appLeader.GetStore().SetTest(strconv.Itoa(idx), idx)
				//	f.Error()
				//	w.Done()
				//})
			}(i)
			//runChan.Handle(i, 10*time.Second, func(err error) {
			//	f := appLeader.GetStore().SetTest(strconv.Itoa(i), i)
			//	f.Error()
			//	w.Done()
			//})
			//func(idx int) {
			//	f := appLeader.GetStore().SetTest(strconv.Itoa(idx), idx)
			//	f.Error()
			//	w.Done()
			//}(i)
		}
		w.Wait()
		te.Call("end")
	}
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			go func() {
				n := time.Now().UnixNano()
				logrus.Infof("begin,%d", n)
				clientFunc()
				logrus.Infof("end,%d", time.Now().UnixNano()-n)
				t.Logf("singleApp %d ms", (time.Now().UnixNano()-n)/1e6)
				appLeader.Stop()
			}()
		}
	})
	appLeader.Start()
	exitWait.Wait()
}
func Test_RaftCluster_Apply(t *testing.T) {
	genTestClusterApp2GrpcYaml()

	var exitWait common.GracefulExit
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	if err := appLeader.Init(leaderYaml); err != nil {
		t.Errorf("appLeader Init error,%s", err.Error())
		return
	}
	clientFunc := func() {
		var w sync.WaitGroup
		var te common.TimeElapse
		te.Call("begin")
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				//var req SetReq
				//key := strconv.Itoa(idx)
				//req.A = map[string]*TestUnit{}
				//req.A[key] = &TestUnit{
				//	A: key,
				//}
				//f := app.NewReplyFuture(context.WithValue(context.Background(), "hash", key), &req, &SetRsp{TimeLine: []*TimeLineUnit{}})
				//appLeader.GRpcHandle(f)
				f := appLeader.GetStore().Set(strconv.Itoa(idx), idx)
				f.Error()
				w.Done()
				//runChan.Handle(idx, 10*time.Second, func(err error) {
				//	f := appLeader.GetStore().SetTest(strconv.Itoa(idx), idx)
				//	f.Error()
				//	w.Done()
				//})
			}(i)
			//runChan.Handle(i, 10*time.Second, func(err error) {
			//	f := appLeader.GetStore().SetTest(strconv.Itoa(i), i)
			//	f.Error()
			//	w.Done()
			//})
			//func(idx int) {
			//	f := appLeader.GetStore().SetTest(strconv.Itoa(idx), idx)
			//	f.Error()
			//	w.Done()
			//}(i)
		}
		w.Wait()
		te.Call("end")
	}
	var once sync.Once
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			appFollower := app.NewMainApp(app.CreateApp("test"), &exitWait)
			appFollower.OnLeaderChg.Add(func(i interface{}) {
				if len(i.(string)) == 0 {
					return
				}
				appFollower2 := app.NewMainApp(app.CreateApp("test"), &exitWait)
				appFollower2.OnLeaderChg.Add(func(i interface{}) {
					if len(i.(string)) == 0 {
						return
					}
					go func() {
						n := time.Now().UnixNano()
						once.Do(clientFunc)
						t.Logf("clusterApp2 %f ms", float64(time.Now().UnixNano()-n)/1e6)
						appLeader.Stop()
						appFollower.Stop()
						appFollower2.Stop()
					}()
				})
				if err := appFollower2.Init(follower2Yaml); err != nil {
					t.Errorf("appFollower2 Init error,%s", err.Error())
					appLeader.Stop()
					appFollower.Stop()
					return
				}
				appFollower2.Start()
			})
			if err := appFollower.Init(followerYaml); err != nil {
				t.Errorf("appFollower Init error,%s", err.Error())
				appLeader.Stop()
				return
			}
			appFollower.Start()
		}
	})
	appLeader.Start()
	exitWait.Wait()
}

func Test_markSingleAppHttp(t *testing.T) {
	genTestSingleYaml()
	singleAppTemplate(t, func() {
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func() {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, "http://127.0.0.1:18320", "/test.SetReq", key, 1, true)
				httpClient(t, "http://127.0.0.1:18320", "/test.GetReq", key, 1, true)
				httpClient(t, "http://127.0.0.1:18320", "/test.DelReq", key, 1, true)
				httpClient(t, "http://127.0.0.1:18320", "/test.GetReq", key, 1, true)
			}()
		}
		w.Wait()
	})
}
func Test_ClusterAppGRpc(t *testing.T) {
	common.OpenDebugLog()
	genTestClusterAppGrpcYaml()
	clusterAppTemplate(t, func() {
		c := []*testGRpcClient{newClient("127.0.0.1:18310"), newClient("127.0.0.1:18311")}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				//runClient(t, c[idx%len(c)], "get", key, 1, true)
				//runClient(t, c[idx%len(c)], "get", key, 1, true)
				runClient(t, c[idx%len(c)], "get", key, 1, true)
				runClient(t, c[idx%len(c)], "get", key, 1, true)

				runClient(t, c[idx%len(c)], "set", key, 1, true)
				runClient(t, c[idx%len(c)], "set", key, 1, true)
				//runClient(t, c[idx%len(c)], "set", key, 1, false)
				//runClient(t, c[idx%len(c)], "set", key, 1, false)
			}(i)
		}
		w.Wait()
	})
}

func Test_ClusterAppHttp(t *testing.T) {
	genTestClusterAppGrpcYaml()
	clusterAppTemplate(t, func() {
		addr := []string{"http://127.0.0.1:18320", "http://127.0.0.1:18321"}
		var w sync.WaitGroup
		w.Add(count)
		for i := 0; i < count; i++ {
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/test.SetReq", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, true)
				httpClient(t, addr[idx%len(addr)], "/test.DelRdq", key, 1, true)
			}(i)
		}
		w.Wait()
	})
}

func httpClient2(t *testing.T, idx int, doneWait *sync.WaitGroup, addr, path string, key string, writeTimes int, hash bool) {
	logrus.Infof("httpClient2,%d", idx)
	doneWait.Add(1)
	defer func() {
		doneWait.Done()
		logrus.Infof("doneWait.Done(),%d", idx)
	}()
	var req interface{}
	switch path {
	case "/test.GetReq":
		req = &GetReq{
			A: key,
		}
	case "/test.SetReq":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		req = &SetReq{
			A: A,
		}
	case "/test.DelReq":
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
		logrus.Infof("http rsp err %d,%s", idx, err.Error())
	} else {
		bb := make([]byte, 4096)
		rsp.Body.Read(bb)
		//t.Logf("http rsp : %s", string(bb))
	}
}
func Test_ClusterApp2GRpc(t *testing.T) {
	genTestClusterApp2GrpcYaml()
	clusterApp2Template(t, func() {
		c := []*testGRpcClient{newClient("127.0.0.1:18310")}
		var w sync.WaitGroup

		for i := 0; i < 10000; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c[idx%len(c)], "set", key, 1, true)
				runClient(t, c[idx%len(c)], "del", key, 1, true)
				runClient(t, c[idx%len(c)], "set", key, 1, true)
				runClient(t, c[idx%len(c)], "del", key, 1, true)
			}(i)
		}
		w.Wait()
	})
}

func Test_ClusterApp2Http(t *testing.T) {
	genTestClusterApp2GrpcYaml()
	clusterApp2Template(t, func() {
		addr := []string{"http://127.0.0.1:18320", "http://127.0.0.1:18321", "http://127.0.0.1:18322"}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				httpClient(t, addr[idx%len(addr)], "/test.SetReq", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/test.DelReq", key, 1, false)
				httpClient(t, addr[idx%len(addr)], "/test.GetReq", key, 1, false)
			}(i)
		}
		w.Wait()
	})
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
	case "/test/GetReq":
		req = &GetReq{
			A: key,
		}
	case "/test.SetReq":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		req = &SetReq{
			A: A,
		}
	case "/test.DelReq":
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
		t.Logf("http rsp : %s", string(bb))
	}
}

var errlog sync.Once

var cnt, totalTime int64

func printAveQ(t *testing.T) {
	t.Logf("cnt %d,total %d,ave %v", cnt, totalTime, totalTime/cnt)
}

var totalTimeLine []TimeLineUnit
var totalTimeLineCnt int64

func printTotalTimeLine() {
	timeline := make([]string, 0)
	for i, _ := range totalTimeLine {
		totalTimeLine[i].Timeline = totalTimeLine[i].Timeline / totalTimeLineCnt
		timeline = append(timeline, fmt.Sprintf("%s %d", totalTimeLine[i].Tag, totalTimeLine[i].Timeline))
	}
	logrus.Infof("totalCnt %d,%s", totalTimeLineCnt, strings.Join(timeline, ","))
}

func runClient(t *testing.T, c *testGRpcClient, cmd string, key string, writeTimes int, hash bool) {
	n := time.Now().UnixNano()
	defer func() {
		atomic.AddInt64(&totalTime, int64((time.Now().UnixNano()-n)/1e6))
		atomic.AddInt64(&cnt, 1)
		//if _n := time.Now().UnixNano() - n; _n > 3*1e9 {
		//	t.Logf("runClient,%s,%s,%vms", cmd, key, _n/1e6)
		//}
	}()
	header := &Header{}
	if hash {
		header.Hash = key
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	switch cmd {
	case "get":
		_, e := c.Get().GetRequest(ctx, &GetReq{
			Header: header,
			A:      key,
			B:      0,
			C:      0,
		})
		if e != nil {
			//errlog.Do(func() {
			logrus.Errorf("get err ,%s,%s", key, e.Error())
			//})
			//t.Logf("send err ,%s", e.Error())
		} else {
			//t.Logf("getrsp")
		}
	case "set":
		A := make(map[string]*TestUnit, 0)
		for i := 0; i < writeTimes; i++ {
			A[fmt.Sprintf("%s_%d", key, i)] = newTestUnit()
		}
		rsp, e := c.Get().SetRequest(ctx, &SetReq{
			Header: header,
			A:      A,
			Timeline: []*TimeLineUnit{&TimeLineUnit{
				Tag:      "Send",
				Timeline: time.Now().UnixNano(),
			}},
		})
		if e != nil {
			//errlog.Do(func() {
			logrus.Errorf("set err ,%s,%s", key, e.Error())
			//})
		} else {
			if app.DebugTraceFutureLine {

				rsp.TimeLine = append(rsp.TimeLine, &TimeLineUnit{
					Tag:      "Result",
					Timeline: time.Now().UnixNano(),
				})
				if totalTimeLine == nil {
					totalTimeLine = make([]TimeLineUnit, len(rsp.TimeLine))
					totalTimeLineCnt = 0
				}
				totalTimeLineCnt++
				var dif int64
				timeline := make([]string, 0)
				for i := len(rsp.TimeLine) - 1; i > 0; i-- {
					from := rsp.TimeLine[i-1]
					to := rsp.TimeLine[i]
					dif += to.Timeline - from.Timeline
					timeline = append(timeline, fmt.Sprintf("%s ==> %s %d", from.Tag, to.Tag, to.Timeline-from.Timeline))
					totalTimeLine[i].Tag = fmt.Sprintf("%s ==> %s", from.Tag, to.Tag)
					totalTimeLine[i].Timeline += to.Timeline - from.Timeline
				}
				//if dif > 3000*1e3 {
				t.Logf("Timeline(%d): %s", dif, strings.Join(timeline, " , "))
				//}
			}
			//t.Logf("setrsp, %v", *rsp)
		}
	case "del":
		_, e := c.Get().DelRequest(ctx, &DelReq{
			Header: header,
			A:      key,
		})
		if e != nil {
			//errlog.Do(func() {
			logrus.Errorf("del err ,%s,%s", key, e.Error())
			//})
		} else {
			//t.Logf("delrsp, %v", *rsp)
		}
	default:
	}
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
