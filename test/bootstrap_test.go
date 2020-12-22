package test

import (
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/raft"

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
		addrs = append(addrs, fmt.Sprintf("127.0.0.1:%d", 18330+i))
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
			t.Logf("clusterAppBootstrapExpect %f ms", float64(time.Now().UnixNano()-n)/1e6)
			stop()
		})
	}
	for i := 0; i < nodeNum; i++ {
		yaml := fmt.Sprintf("cache/node%d.yaml", i)
		genYamlBase(yaml, false, i, true, func(configure *app.Configure) {
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
		exitWait.Done(1, "")
	}
	var once sync.Once
	deal := func(i interface{}) {
		if len(i.(string)) == 0 {
			return
		}
		go once.Do(func() {
			n := time.Now().UnixNano()
			clientFunc()
			t.Logf("clusterAppBootstrapExpectJoinFile %f ms", float64(time.Now().UnixNano()-n)/1e6)
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
	exitWait.Add(1, "")
	for i := 0; i < nodeNum; i++ {
		go func(i int) {
			time.Sleep(time.Duration(rand.Intn(nodeNum*1000)) * time.Millisecond)
			yaml := fmt.Sprintf("cache/node%d.yaml", i)
			genYamlBase(yaml, false, i, true, func(configure *app.Configure) {
				configure.Raft.BootstrapExpect = nodeNum
				configure.JoinFile = joinFile
				configure.JoinAddr = ""
			})
			appNode[i] = app.NewMainApp(app.CreateApp("test"), &exitWait)
			appNode[i].OnLeaderChg.Add(deal)
			if err := appNode[i].Init(yaml); err != nil {
				t.Errorf("appNode%d Init error,%s", i, err.Error())
				stop()
				return
			}
			appNode[i].Start()

			addrChan <- fmt.Sprintf("127.0.0.1:%d", 18330+i)
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
	clusterAppBootstrapExpectJoinFile(t, 3, func() {
		t.Logf("Test_bootstrapExpectJoinFile Ok")
		time.Sleep(3 * time.Second)
	})
}

type fsmTest struct {
}

func (th *fsmTest) Apply(l *raft.Log) interface{} {
	return nil
}
func (th *fsmTest) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}
func (th *fsmTest) Restore(io.ReadCloser) error {
	return nil
}
func Test_bootstrapConflict(t *testing.T) {
	var ras []*raft.Raft = make([]*raft.Raft, 0)
	var configuration raft.Configuration
	configuration.Servers = make([]raft.Server, 0)
	for i := 0; i < 3; i++ {
		config := raft.DefaultConfig()
		config.LogOutput = os.Stdout
		config.LocalID = raft.ServerID(fmt.Sprintf("n%d", i))
		config.SnapshotInterval = 100 * time.Second
		config.ShutdownOnRemove = false
		config.MaxAppendEntries = 1000
		logStore := raft.NewInmemStore()
		stableStore := raft.NewInmemStore()
		snapshot := raft.NewInmemSnapshotStore()
		ip := fmt.Sprintf("127.0.0.1:1111%d", i)
		addr, _ := net.ResolveTCPAddr("tcp", ip)
		transport, _ := raft.NewTCPTransport(ip, addr, 3, 4*time.Second, os.Stdout)
		ra1, _ := raft.NewRaft(config, &fsmTest{}, logStore, stableStore, snapshot, transport)
		ras = append(ras, ra1)
		configuration.Servers = append(configuration.Servers, raft.Server{
			ID:      config.LocalID,
			Address: raft.ServerAddress(ip),
		})
	}
	if err := ras[0].BootstrapCluster(configuration).Error(); err != nil {
		t.Errorf("Boot err,%s", err.Error())
	}
	time.Sleep(5 * time.Second)
	t.Log("------------------BeginRemove------------------------")
	var rmId int
	var leader *raft.Raft
	for i, ra := range ras {
		if ra.State() == raft.Leader {
			leader = ra
			rmId = (i + 1) % len(ras)
			rmNode := fmt.Sprintf("n%d", rmId)
			t.Logf("Remove %s", rmNode)
			if err := ra.RemoveServer(raft.ServerID(rmNode), 0, 0).Error(); err != nil {
				t.Errorf("Remove err,%s", err.Error())
			}
			if err := ras[rmId].Shutdown().Error(); err != nil {
				t.Errorf("Shutdown err,%s", err.Error())
			}
			break
		}
	}

	time.Sleep(time.Second)
	t.Log("------------------ReJoin------------------------")

	config := raft.DefaultConfig()
	config.LogOutput = os.Stdout
	config.LocalID = raft.ServerID(fmt.Sprintf("n%d", rmId))
	config.SnapshotInterval = 100 * time.Second
	config.ShutdownOnRemove = false
	config.MaxAppendEntries = 1000
	logStore := raft.NewInmemStore()
	stableStore := raft.NewInmemStore()
	snapshot := raft.NewInmemSnapshotStore()
	ip := fmt.Sprintf("127.0.0.1:11114")
	addr, _ := net.ResolveTCPAddr("tcp", ip)
	transport, _ := raft.NewTCPTransport(ip, addr, 3, 4*time.Second, os.Stdout)
	ra, _ := raft.NewRaft(config, &fsmTest{}, logStore, stableStore, snapshot, transport)
	go func() {
		t.Log("Reboot")
		configuration.Servers[rmId].Address = raft.ServerAddress(ip)
		if err := ra.BootstrapCluster(configuration).Error(); err != nil {
			t.Errorf("Boot err,%s", err.Error())
		}
	}()
	go func() {
		time.Sleep(5000 * time.Millisecond)
		t.Log("AddVoter")
		if err := leader.AddVoter(config.LocalID, raft.ServerAddress(ip), 0, 0).Error(); err != nil {
			t.Errorf("AddVoter err,%s", err.Error())
		}
	}()

	//time.Sleep(1 * time.Millisecond)
	//t.Log("Leader ", ra.Leader())
	//t.Log("------------------ReBoot------------------------")
	//
	//configuration.Servers[rmId].Address = raft.ServerAddress(ip)
	//if err := ra.BootstrapCluster(configuration).Error(); err != nil {
	//	t.Errorf("Boot err,%s", err.Error())
	//} else {
	//	t.Log("Reboot success")
	//}
	//t.Log("Leader ", ra.Leader())

	time.Sleep(20 * time.Second)

}
