package common

import "time"

type Ticker struct {
	exit chan struct{}
}

func (th *Ticker) Stop() {
	th.exit <- struct{}{}
}

func NewTicker(duration time.Duration, cb func()) *Ticker {
	return NewTickerWithGo(duration, cb, &DefaultGoFunc{})
}

func NewTickerWithGo(duration time.Duration, cb func(), goFunc GoFunc) *Ticker {
	ticker := &Ticker{
		exit: make(chan struct{}, 1),
	}
	t := time.NewTicker(duration)
	if goFunc == nil {
		goFunc = &DefaultGoFunc{}
	}
	goFunc.Go(func() {
		defer t.Stop()
		for {
			select {
			case <-t.C:
				cb()
			case <-ticker.exit:
				return
			}
		}
	})
	return ticker
}

func NewTickerImm(duration time.Duration, cb func()) *Ticker {
	cb()
	return NewTicker(duration, cb)
}

type Timer struct {
	exit chan struct{}
}

func (th *Timer) Stop() {
	th.exit <- struct{}{}
}
func NewTimer(duration time.Duration, cb func()) *Timer {
	return NewTimerWithGo(duration, cb, &DefaultGoFunc{})
}
func NewTimerWithGo(duration time.Duration, cb func(), goFunc GoFunc) *Timer {
	timer := &Timer{
		exit: make(chan struct{}, 1),
	}
	t := time.NewTimer(duration)
	if goFunc == nil {
		goFunc = &DefaultGoFunc{}
	}
	goFunc.Go(func() {
		defer timer.Stop()
		select {
		case <-t.C:
			cb()
		case <-timer.exit:
			break
		}
	})
	return timer
}
func NewTimerImm(duration time.Duration, cb func()) *Timer {
	cb()
	return NewTimer(duration, cb)
}
