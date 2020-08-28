package common

import (
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type RunChanType chan func()
type QPS struct {
	TotalTime int64
	Cnt       int64
}
type LogicChan struct {
	GracefulGoFunc
	runChan  []RunChanType
	exitChan []chan struct{}
	qps      []QPS
	goFunc   GoFunc
}

func (th *LogicChan) Init(maxChan int, goFunc GoFunc) {
	if maxChan <= 0 {
		maxChan = 1
	}
	th.goFunc = goFunc
	th.runChan = make([]RunChanType, maxChan)
	th.exitChan = make([]chan struct{}, maxChan)
	th.qps = make([]QPS, maxChan)
	for i := 0; i < len(th.runChan); i++ {
		th.exitChan[i] = make(chan struct{}, 1)
		th.runChan[i] = make(RunChanType, 4096)
	}
	th.UpdateExitWait(&GracefulExit{})
}
func (th *LogicChan) Get() RunChanType {
	return th.runChan[0]
}
func (th *LogicChan) HandleNoHash(h func()) {
	th.Handle(0, h)
}
func (th *LogicChan) HandleWithHash(hash string, h func()) {
	hs := fnv.New32()
	_, _ = hs.Write([]byte(hash))
	th.Handle(int(hs.Sum32()), h)
}
func (th *LogicChan) Handle(hash int, h func()) {
	if th.runChan == nil {
		th.goFunc.Go(h)
		return
	}
	i := hash % len(th.runChan)
	th.Go(func() {
		th.runChan[i] <- h
	})
}
func (th *LogicChan) Start() {
	defer func() {
		th.runChan = nil
		th.exitChan = nil
	}()
	var w sync.WaitGroup
	for i := 0; i < len(th.runChan); i++ {
		w.Add(1)
		th.goFunc.GoN(func(p ...interface{}) {
			idx := p[0].(int)
			defer func() {
				close(th.runChan[idx])
				close(th.exitChan[idx])
				w.Done()
			}()
			for {
				select {
				case f := <-th.runChan[idx]:
					deal := func(fn func()) {
						n := time.Now().UnixNano()
						fn()
						th.qps[idx].TotalTime += time.Now().UnixNano() - n
						th.qps[idx].Cnt++
					}
					deal(f)
					if len(th.runChan[idx]) > 0 {
						break
					}
				case <-th.exitChan[idx]:
					logrus.Debugf("idx(%d) exit", idx)
					return
				}
			}
		}, i)
	}
	w.Wait()
}
func (th *LogicChan) Stop() {
	th.Wait()
	if th.exitChan != nil {
		for i := 0; i < len(th.exitChan); i++ {
			th.exitChan[i] <- struct{}{}
		}
	}
}
func (th *LogicChan) GetQPS() []string {
	info := make([]string, len(th.qps)+1)
	var cnt, totalTime, q int64
	for i, qps := range th.qps {
		if qps.Cnt > 0 && qps.TotalTime > 0 {
			cnt += qps.Cnt
			totalTime += qps.TotalTime
			q += 1e9 / (qps.TotalTime / qps.Cnt)
			info[i] = fmt.Sprintf("chan(%d) call %d,totaltime %d,per %d nans,qps=%d", i, qps.Cnt, qps.TotalTime, qps.TotalTime/qps.Cnt, 1e9/(qps.TotalTime/qps.Cnt))
		}
	}
	if cnt > 0 && totalTime > 0 {
		info[len(th.qps)] = fmt.Sprintf("total call %d,totaltime %d,per %d nans,qps=%d", cnt, totalTime, totalTime/cnt, q)
	}
	return info
}
