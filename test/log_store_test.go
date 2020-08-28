package test

import (
	"math/rand"
	"strconv"
	"sync"
	"testing"

	"git.shiyou.kingsoft.com/infra/go-raft/store"

	"git.shiyou.kingsoft.com/infra/go-raft/common"
)

func Test_LogStore(t *testing.T) {
	common.OpenDebugLog()
	capacity := 500
	count = 50000
	leaderYaml = genYamlBase(leaderYaml, true, 0, true, func(configure *common.Configure) {
		configure.LogCacheCapacity = capacity
	})
	followerYaml = genYamlBase(followerYaml, false, 1, true, func(configure *common.Configure) {
		configure.LogCacheCapacity = capacity
	})
	clusterAppTemplate(t, func() {
		c := []*testGRpcClient{newClient("127.0.0.1:18310")}
		var w sync.WaitGroup
		for i := 0; i < count; i++ {
			w.Add(1)
			go func(idx int) {
				//defer w.Done()
				key := strconv.Itoa(rand.Int())
				runClient(t, c[idx%len(c)], "set", key, 1, true)
				runClient(t, c[idx%len(c)], "set", key, 1, true)
			}(i)
		}
		w.Wait()
	})
}
func Test_logStoreCrashRestore(t *testing.T) {
	store.OpenLogStoreDebugCrash()
	//common.OpenDebugLog()
	logCacheCapacity = 10
	crash(t, 3, 12)
}
