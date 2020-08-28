package test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func randStringSlice(ss []string) {
	if len(ss) < 2 {
		return
	}
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < len(ss); i++ {
		idx := rand.Intn(len(ss))
		if idx > 0 {
			ss[0], ss[idx] = ss[idx], ss[0]
		}
	}
}

func getJoinAddr(max int) string {
	addrs := make([]string, 0)
	for i := 0; i < max; i++ {
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 18310+i))
	}
	//随机打乱顺序
	randStringSlice(addrs)
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

func clusterAppBootstrapExpectJoinFile(t *testing.T, nodeNum int, clientFunc func()) {
	joinFile := "cache/join_addr.txt"
	ioutil.WriteFile(joinFile, []byte(""), os.ModePerm)
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
		exitWait.Done("")
	}
	var once sync.Once
	deal := func(i interface{}) {
		if len(i.(string)) == 0 {
			return
		}
		go once.Do(func() {
			n := time.Now().UnixNano()
			clientFunc()
			t.Logf("clusterAppBootstrapExpectJoinFile %f ms", float64(time.Now().UnixNano()-n)/10e6)
			stop()
		})
	}
	addrChan := make(chan string, 10)
	go func() {
		addrV := make([]string, 0)
		for {
			select {
			case addr := <-addrChan:
				{
					addrV = append(addrV, addr)
					randStringSlice(addrV)
					ioutil.WriteFile(joinFile, []byte(strings.Join(addrV, "\n")), os.ModePerm)
				}
			}
		}
	}()
	exitWait.Add("")
	for i := 0; i < nodeNum; i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Intn(nodeNum*1000)) * time.Millisecond)
			yaml := fmt.Sprintf("cache/node%d.yaml", i)
			genYamlBase(yaml, false, i, true, func(configure *common.Configure) {
				configure.BootstrapExpect = nodeNum
				configure.JoinFile = joinFile
				configure.JoinAddr = ""
			})
			appNode[i] = app.NewMainApp(app.CreateApp("test"), &exitWait)
			appNode[i].OnLeaderChg.Add(deal)
			if rst := appNode[i].Init(yaml); rst != 0 {
				t.Errorf("appNode%d Init error,%d", i, rst)
				stop()
				return
			}
			appNode[i].Start()

			addrChan <- fmt.Sprintf("127.0.0.1:%d", 18310+i)
		}(i)
	}

	exitWait.Wait()
	for _, a := range appNode {
		if a != nil {
			a.PrintQPS()
		}
	}
}
func Test_bootstrapExpectJoinFile(t *testing.T) {
	clusterAppBootstrapExpectJoinFile(t, 5, func() {
		t.Logf("Test_bootstrapExpectJoinFile Ok")
	})
}
