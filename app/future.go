package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

type FutureCmdType int

var DebugTraceFutureLine = false

const (
	FutureCmdTypeGRpc FutureCmdType = iota //default CmdType
	FutureCmdTypeHttp
	FutureCmdTypeJoin
	FutureCmdTypeRemove
	FutureCmdTypeSynMember
	FutureCmdTypeGracefulStop
	FutureCmdTypeExit
)

type TimelineInfo struct {
	Tag string
	T   int64
}
type TraceFutureInfo struct {
	Timeline []TimelineInfo
	l        sync.RWMutex
	cnt      int32
}

func (th *TraceFutureInfo) AddTimeLine(tag string) {
	if !DebugTraceFutureLine {
		return
	}
	if th.Timeline == nil {
		th.Timeline = make([]TimelineInfo, 0)
	}
	th.Timeline = append(th.Timeline, TimelineInfo{Tag: tag, T: time.Now().UnixNano() / 1e3})
}
func (th *TraceFutureInfo) AddTimelineObj(info TimelineInfo) {
	if !DebugTraceFutureLine {
		return
	}
	if th.Timeline == nil {
		th.Timeline = make([]TimelineInfo, 0)
	}
	th.Timeline = append(th.Timeline, info)
}
func (th *TraceFutureInfo) GetTimeLineDif() string {
	if !DebugTraceFutureLine {
		return ""
	}

	timeline := make([]string, 0)
	var dif int64
	for i := len(th.Timeline) - 1; i > 0; i-- {
		from := th.Timeline[i-1]
		to := th.Timeline[i]
		dif += to.T - from.T
		t := fmt.Sprintf("%s -> %s %d", from.Tag, to.Tag, to.T-from.T)
		timeline = append(timeline, t)
	}
	if dif < 1000 {
		return ""
	}
	return strings.Join(timeline, ",")
}

type ReplyFuture struct {
	ctx         context.Context
	req         interface{}
	responsed   bool
	err         chan error
	e           error
	rsp         interface{}
	rspFuture   HandlerRtv
	prioritized bool
	trans       bool
	cmd         FutureCmdType
	cnt         int
	TraceFutureInfo
}

func (th *ReplyFuture) response(err error) {
	th.err <- err
}
func (th *ReplyFuture) Error() error {
	//defer func() {
	//	if dif := th.GetTimeLineDif(); len(dif) > 0 {
	//		logrus.Debugf("ReplyFuture Timeline : %s", dif)
	//	}
	//}()
	if th.responsed {
		return th.e
	}
	timer := time.After(futureRspTimeout)
	select {
	case <-timer:
		th.e = errTimeout
		logrus.Debugf("ReplyFuture timeout,%v", th.req)
		break
	case th.e = <-th.err:
		break
	}
	th.responsed = true
	return th.e
}
func (th *ReplyFuture) Response() interface{} {
	if !th.responsed {
		return nil
	}
	return th.rsp
}

func NewReplyFuture(ctx context.Context, req, rsp interface{}) *ReplyFuture {
	return &ReplyFuture{
		ctx:       ctx,
		req:       req,
		responsed: false,
		err:       make(chan error),
		rsp:       rsp,
	}
}
func NewReplyFuturePrioritized(ctx context.Context, req, rsp interface{}) *ReplyFuture {
	f := NewReplyFuture(ctx, req, rsp)
	f.prioritized = true
	return f
}

type HttpReplyFuture struct {
	w chan struct{}
}

func (th *HttpReplyFuture) Wait() {
	<-th.w
}
func (th *HttpReplyFuture) Done() {
	th.w <- struct{}{}
}
func NewHttpReplyFuture() *HttpReplyFuture {
	return &HttpReplyFuture{
		w: make(chan struct{}),
	}
}
