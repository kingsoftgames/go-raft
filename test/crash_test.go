package test

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

var appNode []*app.MainApp
var logCacheCapacity = 0

func stop() {
	for _, a := range appNode {
		if a != nil {
			a.Stop()
		}
	}
}
func clusterAppCrash(t *testing.T, nodeNum int, clientFunc func([]*app.MainApp)) {
	if nodeNum > 10 {
		t.Errorf("support <= 10 node")
		return
	}
	var exitWait common.GracefulExit
	appNode = make([]*app.MainApp, nodeNum)

	var once sync.Once
	deal := func(i interface{}) {
		if len(i.(string)) == 0 {
			return
		}
		go once.Do(func() {
			n := time.Now().UnixNano()
			clientFunc(appNode)
			t.Logf("clusterAppCrash %f ms", float64(time.Now().UnixNano()-n)/1e6)
		})
	}
	for i := 0; i < nodeNum; i++ {
		yaml := fmt.Sprintf("cache/node%d.yaml", i)
		genYamlBase(yaml, false, i, true, func(configure *app.Configure) {
			configure.Raft.LogCacheCapacity = logCacheCapacity
			if i == 0 {
				configure.Raft.BootstrapExpect = nodeNum
				configure.JoinAddr = ""
			} else {
				configure.JoinAddr = getJoinAddr(i)
			}
		})
		appNode[i] = app.NewMainApp(app.CreateApp("test"), &exitWait)
		appNode[i].OnLeaderChg.Add(deal)
		if err := appNode[i].Init(yaml); err != nil {
			t.Errorf("appNode%d Init error,%s", i, err.Error())
			stop()
			return
		}
		appNode[i].Start()
	}
	exitWait.Wait()
	for _, a := range appNode {
		if a != nil {
			a.PrintQPS()
		}
	}
}

func crash(t *testing.T, node int, count int) {
	clusterAppCrash(t, node, func(apps []*app.MainApp) {
		var c = make([]*testGRpcClient, 0)
		for i := 0; i < node; i++ {
			c = append(c, newClient(fmt.Sprintf("127.0.0.1:%d", 18310+i)))
		}
		var w sync.WaitGroup
		w.Add(count)
		for i := 0; i < count; i++ {
			go func(idx int) {
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c[idx%len(c)], "set", key, 1, true)
				//runClient(t, c[idx%len(c)], "set", key, 1, true)
			}(i)
		}
		w.Wait()
		t.Logf("begin crash and restart")
		for _, a := range apps {
			if !a.GetStore().IsLeader() {
				a.OnLeaderChg.Add(func(i interface{}) {
					if len(i.(string)) > 0 {
						t.Logf("Test_CrashVoter new Leader %s", i.(string))
					}
				})
			}
		}
		for i, a := range apps {
			if a.GetStore().IsLeader() {
				func(i int, a *app.MainApp) {
					common.NewTimer(3*time.Second, func() {
						t.Logf("[n%d] Begin Stop", i)
						a.Stop()
						//time.Sleep(5 * time.Second)
						//var w sync.WaitGroup
						//w.Add(count)
						//for i := 0; i < count; i++ {
						//	go func(idx int) {
						//		defer w.Done()
						//		key := strconv.Itoa(rand.Int())
						//		runClient(t, c[idx%len(c)], "set", key, 1, true)
						//		//runClient(t, c[idx%len(c)], "set", key, 1, true)
						//	}(i)
						//}
						//w.Wait()

						common.NewTimer(5*time.Second, func() {
							t.Logf("[n%d] Test_CrashVoter Restart ", i)
							var exitWait common.GracefulExit
							appNode[i] = app.NewMainApp(app.CreateApp("test"), &exitWait)
							appNode[i].OnLeaderChg.Add(func(i interface{}) {
								if len(i.(string)) > 0 {
									t.Logf("Test_CrashVoter Restart End, leader %s", i.(string))
									//stop()
								}
							})
							if err := appNode[i].Init(fmt.Sprintf("cache/node%d.yaml", i)); err != nil {
								t.Errorf("appNode%d Init error,%s", i, err.Error())
								stop()
								return
							}
							appNode[i].Start()
							exitWait.Wait()
						})
					})
				}(i, a)
				break
			}
		}
	})
}

func Test_CrashVoter(t *testing.T) {
	crash(t, 3, 50)
}

func Test_CrashNotify(t *testing.T) {
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *app.Configure) {
	})
	notify := "cache/crash_notify.sh"
	content := "#!/bin/bash\nmore $1"
	if runtime.GOOS == "windows" {
		notify = "crash_notify.bat"
		content = "more %1"
	}
	ioutil.WriteFile(notify, []byte(content), os.ModePerm)
	singleAppTemplate(t, func() {
		newClient("127.0.0.1:18310").Get().CrashRequest(context.Background(), &CrashReq{
			Header: &Header{},
		})
	})
}
