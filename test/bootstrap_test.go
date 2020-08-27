package test

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func getJoinAddr(max int) string {
	addrs := make([]string, 0)
	for i := 0; i < max; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 18310+i))
	}
	//随机下打乱第一个
	if len(addrs) > 1 {
		rand.Seed(time.Now().UnixNano())
		idx := rand.Intn(len(addrs))
		if idx > 0 {
			addrs[0], addrs[idx] = addrs[idx], addrs[0]
		}
	}
	return strings.Join(addrs, ",")
}

func clusterAppBootstrapExpect(t *testing.T, nodeNum int, clientFunc func()) {
	if nodeNum > 10 {
		t.Errorf("support <= 10 node")
		return
	}
	var exitWait common.GracefulExit
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
			t.Logf("clusterAppBootstrapExpect %f ms", float64(time.Now().UnixNano()-n)/10e6)
			stop()
		})
	}
	for i := 0; i < nodeNum; i++ {
		yaml := fmt.Sprintf("cache/node%d.yaml", i)
		genYamlBase(yaml, false, i, true, func(configure *common.Configure) {
			if i == 0 {
				configure.BootstrapExpect = nodeNum
				configure.JoinAddr = ""
			} else {
				configure.JoinAddr = getJoinAddr(i)
			}
		})
		appNode[i] = app.NewMainApp(app.CreateApp("test"), &exitWait)
		appNode[i].OnLeaderChg.Add(deal)
		if rst := appNode[i].Init(yaml); rst != 0 {
			t.Errorf("appNode%d Init error,%d", i, rst)
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

func Test_bootstrapExpect(t *testing.T) {
	clusterAppBootstrapExpect(t, 3, func() {
		t.Logf("Test_bootstrapExpect Ok")
	})
}
