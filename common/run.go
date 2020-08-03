package common

import (
	"hash/fnv"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type RunChanType chan func()

type LogicChan struct {
	runChan  []RunChanType
	exitChan []chan os.Signal
	w        sync.WaitGroup
}

func (th *LogicChan) Init(maxChan int) {
	if maxChan <= 0 {
		maxChan = 1
	}
	th.runChan = make([]RunChanType, maxChan)
	th.exitChan = make([]chan os.Signal, maxChan)
	for i := 0; i < len(th.runChan); i++ {
		th.exitChan[i] = make(chan os.Signal, 1)
		th.runChan[i] = make(RunChanType, 4096)
	}
}
func (th *LogicChan) Get() RunChanType {
	return th.runChan[0]
}
func (th *LogicChan) HandleWithHash(hash string, h func()) {
	hs := fnv.New32()
	_, _ = hs.Write([]byte(hash))
	th.Handle(int(hs.Sum32()), h)
}
func (th *LogicChan) Handle(hash int, h func()) {
	if th.runChan == nil {
		go h()
		return
	}
	i := hash % len(th.runChan)
	go func() {
		th.runChan[i] <- h
	}()
}
func (th *LogicChan) Start() {
	defer func() {
		th.runChan = nil
		th.exitChan = nil
	}()
	for i := 0; i < len(th.runChan); i++ {
		signal.Notify(th.exitChan[i], os.Interrupt)
		signal.Notify(th.exitChan[i], os.Kill)
		th.w.Add(1)
		go func(idx int) {
			var totalTime, qps int64
			defer func() {
				if totalTime > 0 && qps > 0 {
					logrus.Infof("Chan(%d) call %d,time,%d,per %d nans,qps=%d", idx, qps, totalTime, totalTime/qps, 1e9/(totalTime/qps))
				}
				close(th.runChan[idx])
				close(th.exitChan[idx])
				th.w.Done()
			}()
			for {
				select {
				case f := <-th.runChan[idx]:
					n := time.Now().UnixNano()
					f()
					totalTime = totalTime + (time.Now().UnixNano() - n)
					qps++
				case <-th.exitChan[idx]:
					return
				}
			}
		}(i)
	}
	th.w.Wait()
}
func (th *LogicChan) Stop() {
	if th.exitChan != nil {
		for i := 0; i < len(th.exitChan); i++ {
			th.exitChan[i] <- os.Kill
		}
	}
}
