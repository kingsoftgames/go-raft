package main

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/test"

	"git.shiyou.kingsoft.com/infra/go-raft/app"
)

var cli *bool
var cnt *int
var addr *string
var print *bool
var timeout *int
var hash *bool
var cmd *string
var cntUnit *int
var key *string
var debug *bool
var local *bool
var naked *bool

func main() {
	testApp := kingpin.New(os.Args[0], "pressure test raft")
	cli := testApp.Flag("cli", "whether run as client").Bool()
	cnt = testApp.Flag("cnt", "count for send query (cnt*unit)").Default("1").Int()
	cntUnit = testApp.Flag("unit", "unit size for query").Default("1000").Int()
	addr = testApp.Flag("addr", "addr for api").Default("127.0.0.1:18310").String()
	print = testApp.Flag("print", "print result").Bool()
	timeout = testApp.Flag("timeout", "timeout for grpc").Default("5000").Int()
	hash = testApp.Flag("hash", "whether use hash when query").Bool()
	cmd = testApp.Flag("cmd", "cmd for query,can multi cmd split by ','").Default("set").String()
	key = testApp.Flag("key", "key for get','").String()
	debug = testApp.Flag("debug", "whether debug for test").Bool()
	local = testApp.Flag("local", "local test").Bool()
	naked = testApp.Flag("naked", "only when local=true take affect").Bool()
	if cmd, err := testApp.Parse(os.Args[1:]); err != nil {
		logrus.Fatal("parse flag err,%s", err.Error())
	} else {
		logrus.Infof("test flag: %s", cmd)
	}
	if *cli {
		client()
	} else {
		if *debug {
			app.DebugTraceFutureLine = true
		}
		app.RunMain("test")
	}
}
func init() {
	rand.Seed(time.Now().UnixNano())
}
func client() {
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
	if *local {
		test.GRpcLocalQuery(*addr, *cmd, *cnt, *naked, time.Duration(*timeout)*time.Millisecond)
		return
	}
	if len(*key) > 0 {
		keys := strings.Split(*key, ",")
		for _, k := range keys {
			test.GRpcQuery(*addr, "get", k, 1, *hash, time.Duration(*timeout)*time.Millisecond, *print)
		}
		return
	}
	cmds := strings.Split(*cmd, ",")
	var totalTime int64 = 0
	var timeoutCnt int64
	var min, max int64
	var w sync.WaitGroup
	bg := time.Now().UnixNano()
	for i := 0; i < *cnt; i++ {
		for j := 0; j < *cntUnit; j++ {
			w.Add(1)
			go func() {
				n := time.Now().UnixNano() / 1e3
				defer func() {
					if n > 0 {
						dif := time.Now().UnixNano()/1e3 - n
						if min == 0 || dif < min {
							min = dif
						}
						if max == 0 || dif > max {
							max = dif
						}
						atomic.AddInt64(&totalTime, dif)
					} else {
						atomic.AddInt64(&timeoutCnt, 1)
					}
				}()
				defer w.Done()
				key := strconv.Itoa(rand.Int())
				for _, cmd := range cmds {
					if !test.GRpcQuery(*addr, cmd, key, 1, *hash, time.Duration(*timeout)*time.Millisecond, *print) {
						n = 0
					}
				}
			}()
		}
		w.Wait()
	}
	totalCnt := int64(*cnt * *cntUnit * len(cmds))
	logrus.Infof("total:%d succeed:%d successRate:%v totalTime:%dms ave:%dms min:%dms max:%dms",
		totalCnt,
		totalCnt-timeoutCnt,
		(float32)(totalCnt-timeoutCnt)/(float32)(totalCnt),
		(time.Now().UnixNano()-bg)/1e6,
		totalTime/(totalCnt-timeoutCnt)/1e3,
		min/1e3,
		max/1e3)
}
