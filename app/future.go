package app

import "context"

type ReplyFuture struct {
	ctx       context.Context
	req       interface{}
	responsed bool
	err       chan error
	e         error
	rsp       interface{}
	rspFuture HandlerRtv
}

func (th *ReplyFuture) response(err error) {
	th.err <- err
}
func (th *ReplyFuture) Error() error {
	if th.responsed {
		return th.e
	}
	th.e = <-th.err
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
