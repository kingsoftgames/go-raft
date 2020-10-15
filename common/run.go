package common

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	ErrTimeout  = errors.New("timer out")
	ErrShutdown = errors.New("shutdown")
)

type RunChanType chan func()
type QPS struct {
	TotalTime int64
	Cnt       int64
}
type LogicChan struct {
	GracefulGoFunc
	runChan  []RunChanType
	exitChan chan struct{}
	qps      []QPS
	goFunc   GoFunc
}

func (th *LogicChan) Init(maxChan int, goFunc GoFunc) {
	if maxChan <= 0 {
		maxChan = 1
	}
	th.goFunc = goFunc
	th.runChan = make([]RunChanType, maxChan)
	th.exitChan = make(chan struct{})
	th.qps = make([]QPS, maxChan)
	for i := 0; i < len(th.runChan); i++ {
		th.runChan[i] = make(RunChanType, 4096)
	}
	th.UpdateExitWait(&GracefulExit{})
}
func (th *LogicChan) Get() RunChanType {
	return th.runChan[0]
}
func (th *LogicChan) HandleNoHash(timeout time.Duration, h func(err error)) {
	th.Handle(0, timeout, h)
}
func (th *LogicChan) HandleWithHash(hash string, timeout time.Duration, h func(err error)) {
	hs := fnv.New32()
	_, _ = hs.Write([]byte(hash))
	th.Handle(int(hs.Sum32()), timeout, h)
}
func (th *LogicChan) Handle(hash int, timeout time.Duration, h func(err error)) {
	if th.runChan == nil {
		th.goFunc.Go(func() {
			h(nil)
		})
		return
	}
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}
	i := hash % len(th.runChan)
	th.Go(func() {
		select {
		case <-timer:
			h(ErrTimeout)
		case th.runChan[i] <- func() { h(nil) }:
		case <-th.exitChan:
			h(ErrShutdown)
		}
	})
}
func (th *LogicChan) Start() {
	defer func() {
		th.runChan = nil
	}()
	var w sync.WaitGroup
	for i := 0; i < len(th.runChan); i++ {
		w.Add(1)
		th.goFunc.GoN(func(p ...interface{}) {
			idx := p[0].(int)
			defer func() {
				close(th.runChan[idx])
				w.Done()
			}()
			for {
				select {
				case f := <-th.runChan[idx]:
					deal := func(fn func()) {
						n := time.Now().UnixNano() / 1e3
						fn()
						atomic.AddInt64(&th.qps[idx].TotalTime, (time.Now().UnixNano()/1e3)-n)
						atomic.AddInt64(&th.qps[idx].Cnt, 1)
					}
					deal(f)
					if len(th.runChan[idx]) > 0 {
						break
					}
				case <-th.exitChan:
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
	close(th.exitChan)
}
func (th *LogicChan) GetQPS() []string {
	info := make([]string, len(th.qps)+1)
	var cnt, totalTime, q int64
	for i, _ := range th.qps {
		_cnt := atomic.LoadInt64(&th.qps[i].Cnt)
		_totalTime := atomic.LoadInt64(&th.qps[i].TotalTime)
		if _cnt > 0 && _totalTime > 0 {
			cnt += _cnt
			totalTime += _totalTime
			q += 1e6 / (_totalTime / _cnt)
			info[i] = fmt.Sprintf("chan(%d) call %d,totaltime %d,per %d nans,qps=%d", i, _cnt, _totalTime, _totalTime/_cnt, 1e6/(_totalTime/_cnt))
		}
	}
	if cnt > 0 && totalTime > 0 {
		info[len(th.qps)] = fmt.Sprintf("total call %d,totaltime %d,per %d nans,qps=%d", cnt, totalTime, totalTime/cnt, q)
	}
	return info
}
func (th *LogicChan) GetCnt() (int64, int64) {
	var cnt, totalTime int64
	for i, _ := range th.qps {
		cnt += atomic.LoadInt64(&th.qps[i].Cnt)
		totalTime += atomic.LoadInt64(&th.qps[i].TotalTime)
	}
	return cnt, totalTime
}
