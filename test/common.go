package test

import (
	context "context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/store"

	"gopkg.in/yaml.v2"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"github.com/hashicorp/raft"
	"github.com/sirupsen/logrus"
)

func genConfig(content string, name string) string {
	ioutil.WriteFile(name, []byte(content), os.ModePerm)
	return name
}
func genConfigFromObj(cfg *app.Configure, name string) string {
	b, _ := yaml.Marshal(cfg)
	ioutil.WriteFile(name, b, os.ModePerm)
	return name
}

var leaderYaml = "cache/leader.yaml"
var followerYaml = "cache/follower.yaml"
var follower2Yaml = "cache/follower2.yaml"

func genYamlBase(name string, bootstrap bool, portShift int, storeInMem bool, cb func(configure *app.Configure)) string {
	cfg := &app.Configure{}
	cfg.Raft = new(store.RaftConfigure)
	cfg.Log = new(common.LogConfigure)
	cfg.Debug = new(app.DebugConfigure)
	cfg.Prometheus = new(common.PrometheusConfigure)
	cfg.Alerter = new(common.AlerterConfigure)
	cfg.Raft.Addr = fmt.Sprintf("127.0.0.1:%d", 18300+portShift)
	cfg.GrpcApiAddr = fmt.Sprintf("127.0.0.1:%d", 18310+portShift)
	cfg.HttpApiAddr = fmt.Sprintf("127.0.0.1:%d", 18320+portShift)
	cfg.InnerAddr = fmt.Sprintf("127.0.0.1:%d", 18330+portShift)
	cfg.Raft.Bootstrap = bootstrap
	cfg.Raft.BootstrapExpect = 0
	cfg.JoinAddr = ""
	cfg.TryJoinTime = 3
	if !bootstrap {
		cfg.JoinAddr = "127.0.0.1:18330"
	}
	cfg.HealthCheckIntervalMs = 2000
	cfg.ConnectTimeoutMs = 1000
	cfg.RunChanNum = 1000
	cfg.Raft.NodeId = fmt.Sprintf("n%d", portShift)
	cfg.Raft.StoreInMem = storeInMem
	cfg.Raft.StoreDir = "./cache/store/"
	cfg.Raft.LogCacheCapacity = 1000
	cfg.Log.MaxSize = 100
	cfg.Log.MaxAge = 2
	cfg.Log.Path = "stdout"
	cfg.Log.Level = "DEBUG"
	cfg.Debug.GRpcHandleHash = true
	cfg.Debug.PrintIntervalMs = 5000
	if cb != nil {
		cb(cfg)
	}
	return genConfigFromObj(cfg, name)
}
func genYaml(name string, bootstrap bool, portShift int, storeInMem bool) string {
	return genYamlBase(name, bootstrap, portShift, storeInMem, nil)
}
func genTestSingleYaml() {
	leaderYaml = genYaml(leaderYaml, true, 0, true)
}

func genTestClusterAppGrpcYaml() {
	leaderYaml = genYaml(leaderYaml, true, 0, true)
	followerYaml = genYaml(followerYaml, false, 1, true)
}
func genTestClusterApp2GrpcYaml() {
	leaderYaml = genYaml(leaderYaml, true, 0, true)
	followerYaml = genYaml(followerYaml, false, 1, true)
	follower2Yaml = genYaml(follower2Yaml, false, 2, true)
}
func singleAppTemplate(t *testing.T, clientFunc func()) {
	var exitWait common.GracefulExit
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	if err := appLeader.Init(leaderYaml); err != nil {
		t.Errorf("appLeader Init error,%s", err.Error())
		return
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
	//t.Run("Start", func(t *testing.T) {
	//	appLeader.Start()
	//	t.Parallel()
	//})
	exitWait.Wait()
}

func clusterAppTemplate(t *testing.T, clientFunc func()) {
	var exitWait common.GracefulExit
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	if err := appLeader.Init(leaderYaml); err != nil {
		t.Errorf("appLeader Init error,%s", err.Error())
		return
	}
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			appFollower := app.NewMainApp(app.CreateApp("test"), &exitWait)
			appFollower.OnLeaderChg.Add(func(i interface{}) {
				if len(i.(string)) == 0 {
					return
				}
				go func() {
					n := time.Now().UnixNano()
					clientFunc()
					t.Logf("clusterAppTemplate %f ms", float64(time.Now().UnixNano()-n)/1e6)
					appLeader.Stop()
					appFollower.Stop()
				}()
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
	appLeader.PrintQPS()
}
func clusterApp2Template(t *testing.T, clientFunc func()) {
	var exitWait common.GracefulExit
	appLeader := app.NewMainApp(app.CreateApp("test"), &exitWait)
	if err := appLeader.Init(leaderYaml); err != nil {
		t.Errorf("appLeader Init error,%s", err.Error())
		return
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
					once.Do(func() {
						go func() {
							n := time.Now().UnixNano()
							clientFunc()
							t.Logf("clusterApp2 %f ms", float64(time.Now().UnixNano()-n)/1e6)
							appLeader.PrintQPS()
							appLeader.Stop()
							appFollower.Stop()
							appFollower2.Stop()
						}()
					})

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

type testGRpcClient struct {
	c *app.GRpcClient
}

func (th *testGRpcClient) Get() *testClient {
	return th.c.Get().(*testClient)
}

func newClient(addr string) *testGRpcClient {
	c := app.NewGRpcClient(2000*time.Millisecond, NewTestClient)
	if err := c.Connect(addr, ""); err != nil {
		return nil
	}
	return &testGRpcClient{
		c: c,
	}
}

func newTestUnit() *TestUnit {
	return &TestUnit{
		A: "123",
		B: 1,
		C: 2,
		D: "321",
	}
}
func GRpcLocalQuery(addr string, cmd string, cnt int, naked bool, timeout time.Duration) bool {
	addrs := strings.Split(addr, ",")
	addr = addrs[rand.Intn(len(addrs))]
	c := newClient(addr)
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	rsp, e := c.Get().LocalRequest(ctx, &LocalReq{
		Cmd:   cmd,
		Cnt:   int32(cnt),
		Naked: naked,
	})
	if e != nil {
		logrus.Errorf("local err ,%s", e.Error())
		return false
	}
	logrus.Warnf("local cnt:%d time:%d qps:%v", cnt, rsp.Time, float64(cnt)/(float64(rsp.Time)/1e9))
	return true
}

func GRpcQuery(addr string, cmd string, key string, writeTimes int, hash bool, timeout time.Duration, printResult bool) bool {
	addrs := strings.Split(addr, ",")
	addr = addrs[rand.Intn(len(addrs))]
	c := newClient(addr)
	if c == nil {
		return false
	}
	defer c.c.Close()
	header := &Header{}
	if hash {
		header.Hash = key
	}
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	switch cmd {
	case "get":
		rsp, e := c.Get().GetRequest(ctx, &GetReq{
			Header: header,
			A:      key,
			B:      0,
			C:      0,
		})
		if e != nil {
			logrus.Errorf("get err ,%s,%s", key, e.Error())
			return false
		} else {
			if printResult {
				logrus.Infof("get %v", *rsp)
			}
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
				Timeline: time.Now().UnixNano() / 1e3,
			}},
		})
		if e != nil {
			logrus.Errorf("set err ,%s,%s", key, e.Error())
			return false
		} else {
			if printResult {
				logrus.Infof("set %v", *rsp)
			}
			if app.DebugTraceFutureLine {
				rsp.TimeLine = append(rsp.TimeLine, &TimeLineUnit{
					Tag:      "Result",
					Timeline: time.Now().UnixNano() / 1e3,
				})
				var dif int64
				timeline := make([]string, 0)
				for i := len(rsp.TimeLine) - 1; i > 0; i-- {
					from := rsp.TimeLine[i-1]
					to := rsp.TimeLine[i]
					dif += to.Timeline - from.Timeline
					timeline = append(timeline, fmt.Sprintf("%s ==> %s %d", from.Tag, to.Tag, to.Timeline-from.Timeline))
				}
				if dif > 3000*1e3 {
					logrus.Infof("Timeline(%d): %s", dif, strings.Join(timeline, " , "))
				}
			}
		}
	case "del":
		rsp, e := c.Get().DelRequest(ctx, &DelReq{
			Header: header,
			A:      key,
		})
		if e != nil {
			logrus.Errorf("del err ,%s,%s", key, e.Error())
			return false
		} else {
			if printResult {
				logrus.Infof("del %v", *rsp)
			}
		}
	default:
	}
	return true
}
