package test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func clusterAppVerUpdate(exitWait *common.GracefulExit, t *testing.T, nodeNum int, portshift int, ver string, joinAddr string, clientFunc func()) {
	if nodeNum > 10 {
		t.Errorf("support <= 10 node")
		return
	}

	appNode := make([]*app.MainApp, nodeNum)
	stop := func() {
		for _, a := range appNode {
			if a != nil {
				a.Stop()
			}
		}
	}
	var once sync.Once
	deal := func(i interface{}) {
		if len(i.(string)) == 0 {
			return
		}
		go once.Do(func() {
			n := time.Now().UnixNano()
			clientFunc()
			t.Logf("clusterAppBootstrapExpect %f ms", float64(time.Now().UnixNano()-n)/1e6)
			stop()
		})
	}
	for i := 0; i < nodeNum; i++ {
		yaml := fmt.Sprintf("cache/node_%s_%d.yaml", ver, i)
		genYamlBase(yaml, false, i+portshift, true, func(configure *common.Configure) {
			configure.Ver = ver
			configure.LogCacheCapacity = 200
			if len(joinAddr) > 0 {
				configure.JoinAddr = joinAddr
			} else {
				if i == 0 {
					configure.BootstrapExpect = nodeNum
					configure.JoinAddr = ""
				} else {
					configure.JoinAddr = getJoinAddr(i)
				}
			}
		})
		appNode[i] = app.NewMainApp(app.CreateApp("test"), exitWait)
		appNode[i].OnLeaderChg.Add(deal)
		if rst := appNode[i].Init(yaml); rst != 0 {
			t.Errorf("appNode%d Init error,%d", i+portshift, rst)
			stop()
			return
		}
		appNode[i].Start()
		time.Sleep(3 * time.Second)
	}
	exitWait.Wait()
	for _, a := range appNode {
		if a != nil {
			a.PrintQPS()
		}
	}
}
func Test_VerUpdate(t *testing.T) {
	app.DebugTraceFutureLine = true
	//common.OpenDebugLog()r
	var exitWait common.GracefulExit
	clusterAppVerUpdate(&exitWait, t, 3, 0, "1", "", func() {
		//
		for _i := 0; _i < 1; _i++ {
			time.Sleep(10 * time.Millisecond)
			var w sync.WaitGroup
			c := []*testGRpcClient{newClient("127.0.0.1:18310"), newClient("127.0.0.1:18311"), newClient("127.0.0.1:18312")}
			//c := []*testGRpcClient{newClient("127.0.0.1:18311")}

			for i := 0; i < count; i++ {
				w.Add(1)
				go func(idx int) {
					defer w.Done()
					key := strconv.Itoa(rand.Int())
					runClient(t, c[idx%len(c)], "set", key, 1, true)
					runClient(t, c[idx%len(c)], "get", key, 1, true)
					runClient(t, c[idx%len(c)], "del", key, 1, true)
					runClient(t, c[idx%len(c)], "get", key, 1, true)
				}(i)
			}
			w.Wait()
		}
		printAveQ(t)
		t.Logf(app.GetFutureAve())
		t.Logf("Over")
		time.Sleep(100 * time.Second)
		//
		var exitWait2 common.GracefulExit
		clusterAppVerUpdate(&exitWait2, t, 1, 100, "2", "127.0.0.1:18330", func() {
			t.Logf("Test_VerUpdate Ok")
			var w sync.WaitGroup
			c := []*testGRpcClient{newClient("127.0.0.1:18410")}

			for i := 0; i < count*200; i++ {
				w.Add(1)
				go func(idx int) {
					defer w.Done()
					key := strconv.Itoa(rand.Int())
					runClient(t, c[idx%len(c)], "set", key, 1, true)
					runClient(t, c[idx%len(c)], "get", key, 1, true)
					runClient(t, c[idx%len(c)], "del", key, 1, true)
					runClient(t, c[idx%len(c)], "get", key, 1, true)
				}(i)
			}
			w.Wait()
			printAveQ(t)
			t.Logf(app.GetFutureAve())
			time.Sleep(20 * time.Second)
		})
	})
	printAveQ(t)
}

//
//func (th *ImplementedTestServer) SetRequest(ctx context.Context, req *SetReq) (*SetRsp, error) {
//	f := app.NewReplyFuture(context.WithValue(ctx, "hash", req.Header.Hash), req, &SetRsp{TimeLine: []*TimeLineUnit{}})
//	f.Timeline = make([]app.TimelineInfo, 0)
//	f.Timeline = append(f.Timeline, app.TimelineInfo{Tag: req.Timeline[0].Tag, T: req.Timeline[0].Timeline})
//	f.AddTimeLine("Receive")
//	th.app.GRpcHandle(f)
//	if f.Error() != nil {
//		return nil, f.Error()
//	}
//	rsp := f.Response().(*SetRsp)
//	for _, timeline := range f.Timeline {
//		rsp.TimeLine = append(rsp.TimeLine, &TimeLineUnit{
//			Tag:      timeline.Tag,
//			Timeline: timeline.T,
//		})
//	}
//	rsp.TimeLine = append(rsp.TimeLine, &TimeLineUnit{
//		Tag:      "ResultSend",
//		Timeline: time.Now().UnixNano() / 1e3,
//	})
//	return rsp, nil
//}
