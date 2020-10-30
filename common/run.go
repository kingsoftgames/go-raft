package common

import (
	"errors"
	"fmt"
	"hash/fnv"
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
	ticker   *Ticker
}

func (th *LogicChan) Init(maxChan int) {
	if maxChan <= 0 {
		maxChan = 1
	}
	th.runChan = make([]RunChanType, maxChan)
	th.exitChan = make(chan struct{})
	th.qps = make([]QPS, maxChan)
	for i := 0; i < len(th.runChan); i++ {
		th.runChan[i] = make(RunChanType)
	}
	th.UpdateExitWait(&GracefulExit{})
}
func (th *LogicChan) Get() RunChanType {
	return th.runChan[0]
}
func (th *LogicChan) HandleNoHash(timeout time.Duration, h func(err error)) {
	th.Handle(0, timeout, h)
}
func GetStringSum(hash string) int {
	hs := fnv.New32()
	_, _ = hs.Write([]byte(hash))
	return int(hs.Sum32())
}
func (th *LogicChan) HandleWithHash(hash string, timeout time.Duration, h func(err error)) {
	th.Handle(GetStringSum(hash), timeout, h)
}
func (th *LogicChan) Handle(hash int, timeout time.Duration, h func(err error)) {
	var timer <-chan time.Time
	if timeout > 0 {
		timer = time.After(timeout)
	}
	i := hash % len(th.runChan)
	go func() {
		select {
		case <-timer:
			h(ErrTimeout)
		case th.runChan[i] <- func() { h(nil) }:
		case <-th.exitChan:
			h(ErrShutdown)
		}
	}()
}

//blocked , need call by other goroutine
func (th *LogicChan) Start() {
	logrus.Infof("LogicChan.Start")
	defer func() {
		th.runChan = nil
		logrus.Infof("LogicChan.Start finished")
		if th.ticker != nil {
			th.ticker.Stop()
			th.ticker = nil
		}
	}()
	for i := 0; i < len(th.runChan); i++ {
		th.GoN(func(p ...interface{}) {
			//goid := GoID()
			idx := p[0].(int)
			defer func() {
				logrus.Infof("Close LogicChan(%d)", idx)
				close(th.runChan[idx])
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
				case <-th.exitChan:
					return
				}
			}
		}, i)
	}
	th.Wait()
}
func (th *LogicChan) Stopped() bool {
	return th.ticker != nil
}
func (th *LogicChan) Stop() {
	close(th.exitChan)
	th.ticker = NewTicker(2*time.Second, func() {
		th.exitWait.Print()
	})
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
