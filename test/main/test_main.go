package main

import (
	"flag"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

func main() {
	flag.Parse()
	if *cli {
		client()
	} else {
		app.RunMain()
	}
}
func init() {
	rand.Seed(time.Now().UnixNano())
	cli = flag.Bool("cli", false, "whether run as client")
	cnt = flag.Int("cnt", 1, "count for send query (cnt*unit)")
	cntUnit = flag.Int("unit", 1000, "unit size for query")
	addr = flag.String("addr", "127.0.0.1:18310", "addr for api")
	print = flag.Bool("print", true, "print result")
	timeout = flag.Int("timeout", 5000, "timeout for grpc")
	hash = flag.Bool("hash", true, "whether use hash when query")
	cmd = flag.String("cmd", "set", "cmd for query,can multi cmd split by ','")
	key = flag.String("key", "", "key for get','")
}
func client() {
	logrus.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339Nano})
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
