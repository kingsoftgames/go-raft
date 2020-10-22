package test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func clusterAppGracefulStop(exitWait *common.GracefulExit, t *testing.T, nodeNum int, portshift int, ver string, joinAddr string, clientFunc func()) {
	if nodeNum > 10 {
		t.Errorf("support <= 10 node")
		return
	}

	appNode := make([]*app.MainApp, nodeNum)
	stop := func() {
		for _, a := range appNode {
			if a != nil {
				go a.Stop()
			}
		}
	}
	var once sync.Once
	for i := 0; i < nodeNum; i++ {
		yaml := fmt.Sprintf("cache/node_%s_%d.yaml", ver, i)
		genYamlBase(yaml, false, i+portshift, true, func(configure *common.Configure) {
			configure.Ver = ver
			configure.LogCacheCapacity = 200
			if len(joinAddr) > 0 {
				configure.JoinAddr = joinAddr
			} else {
				if i == 0 {
					configure.BootstrapExpect = 3
					configure.JoinAddr = ""
				} else {
					configure.JoinAddr = getJoinAddr(i)
				}
			}
		})
		appNode[i] = app.NewMainApp(app.CreateApp("test"), exitWait)
		if rst := appNode[i].Init(yaml); rst != 0 {
			t.Errorf("appNode%d Init error,%d", i+portshift, rst)
			stop()
			return
		}
		appNode[i].Start()
		time.Sleep(2 * time.Second)
	}
	go once.Do(func() {
		n := time.Now().UnixNano()
		clientFunc()
		t.Logf("clusterAppBootstrapExpect %f ms", float64(time.Now().UnixNano()-n)/1e6)
		stop()
	})
	exitWait.Wait()
}

func Test_GraceStop1(t *testing.T) {
	//c := make(chan struct{})
	//c1 := make(chan struct{})
	//go func() {
	//	time.Sleep(10 * time.Second)
	//	close(c1)
	//}()
	//for {
	//	select {
	//	case <-c:
	//		t.Log("c")
	//	case <-c1:
	//		t.Log("c1")
	//		return
	//	}
	//}

	var exitWait common.GracefulExit
	clusterAppGracefulStop(&exitWait, t, 3, 0, "1", "", func() {
		t.Logf("Test_GraceStop1")
	})
}
