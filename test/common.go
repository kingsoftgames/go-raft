package test

import (
	context "context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

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
func genConfigFromObj(cfg *common.Configure, name string) string {
	b, _ := yaml.Marshal(cfg)
	ioutil.WriteFile(name, b, os.ModePerm)
	return name
}
func newDefaultCfg() *common.Configure {
	config := common.NewDefaultConfigure()
	return config
}

var leaderYaml = "cache/leader.yaml"
var followerYaml = "cache/follower.yaml"
var follower2Yaml = "cache/follower2.yaml"

func genYamlBase(name string, bootstrap bool, portShift int, storeInMem bool, cb func(configure *common.Configure)) string {
	cfg := newDefaultCfg()
	cfg.RaftAddr = fmt.Sprintf("127.0.0.1:%d", 18300+portShift)
	cfg.GrpcApiAddr = fmt.Sprintf("127.0.0.1:%d", 18310+portShift)
	cfg.HttpApiAddr = fmt.Sprintf("127.0.0.1:%d", 18320+portShift)
	cfg.InnerAddr = fmt.Sprintf("127.0.0.1:%d", 18330+portShift)
	cfg.Bootstrap = bootstrap
	cfg.BootstrapExpect = 0
	cfg.JoinAddr = ""
	cfg.TryJoinTime = 3
	if !bootstrap {
		cfg.JoinAddr = "127.0.0.1:18330"
	}
	cfg.NodeId = fmt.Sprintf("n%d", portShift)
	cfg.StoreInMem = storeInMem
	cfg.StoreDir = "./cache/store/"
	cfg.LogConfig.MaxSize = 100
	cfg.LogConfig.MaxAge = 2
	cfg.LogConfig.Path = "./cache/log"
	cfg.LogConfig.Level = "DEBUG"
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
	if rst := appLeader.Init(leaderYaml); rst != 0 {
		t.Errorf("appLeader Init error,%d", rst)
		return
	}
	appLeader.GetStore().OnStateChg.Add(func(i interface{}) {
		if i.(raft.RaftState) == raft.Leader {
			go func() {
				n := time.Now().UnixNano()
				clientFunc()
				t.Logf("singleApp %f ms", float64(time.Now().UnixNano()-n)/10e6)
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
	if rst := appLeader.Init(leaderYaml); rst != 0 {
		t.Errorf("appLeader Init error,%d", rst)
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
					t.Logf("clusterAppTemplate %f ms", float64(time.Now().UnixNano()-n)/10e6)
					appLeader.Stop()
					appFollower.Stop()
				}()
			})
			if rst := appFollower.Init(followerYaml); rst != 0 {
				t.Errorf("appFollower Init error,%d", rst)
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
	if rst := appLeader.Init(leaderYaml); rst != 0 {
		t.Errorf("appLeader Init error,%d", rst)
		return
	}
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
						clientFunc()
						t.Logf("clusterApp2 %f ms", float64(time.Now().UnixNano()-n)/10e6)
						appLeader.Stop()
						appFollower.Stop()
						appFollower2.Stop()
					}()
				})
				if rst := appFollower2.Init(follower2Yaml); rst != 0 {
					t.Errorf("appFollower2 Init error,%d", rst)
					appLeader.Stop()
					appFollower.Stop()
					return
				}
				appFollower2.Start()
			})
			if rst := appFollower.Init(followerYaml); rst != 0 {
				t.Errorf("appFollower Init error,%d", rst)
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
