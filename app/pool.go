package app

import (
	"container/list"
	"sync"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"
)

const (
	poolMaxConnect = 3
)

type InnerCon struct {
	idx        int
	maxClient  int
	conTimeout int
	addr       string
	clients    *list.List
	l          sync.Mutex
}

func NewInnerCon(addr string, maxClient, conTimeout int) *InnerCon {
	return &InnerCon{
		addr:       addr,
		maxClient:  maxClient,
		conTimeout: conTimeout,
		clients:    list.New(),
	}
}
func (th *InnerCon) GetRaftClient(cb func(client inner.RaftClient)) {
	c := th.Get()
	defer th.BackTo(c)
	if c != nil {
		cb(c.GetClient())
	} else {
		cb(nil)
	}
}
func (th *InnerCon) Get() *innerGRpcClient {
	common.Debugf("[%s]Get", th.addr)
	th.l.Lock()
	front := th.clients.Front()
	if front != nil {
		th.clients.Remove(front)
		th.l.Unlock()
		return front.Value.(*innerGRpcClient)
	}
	th.l.Unlock()
	c := NewInnerGRpcClient(th.conTimeout)
	if err := c.Connect(th.addr); err != nil {
		return nil
	}
	c.Idx = th.idx
	th.idx++
	common.Debugf("[%s]Get,New,%d", th.addr, c.Idx)
	return c
}
func (th *InnerCon) BackTo(c *innerGRpcClient) {
	if c == nil {
		return
	}
	common.Debugf("[%s]BackTo,%d", th.addr, c.Idx)
	th.l.Lock()
	defer th.l.Unlock()
	if th.clients.Len() < th.maxClient {
		th.clients.PushFront(c)
		return
	}
	c.Close()
}
func (th *InnerCon) Close() {
	th.l.Lock()
	defer th.l.Unlock()
	for f := th.clients.Front(); f != nil; f = th.clients.Front() {
		f.Value.(*innerGRpcClient).Close()
	}
}
