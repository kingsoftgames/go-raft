package app

import (
	"sync"
	"sync/atomic"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"
)

const (
	poolMaxConnect = 10
)

type InnerCon struct {
	curNum     int32
	allocNum   int32
	maxClient  int
	conTimeout int
	addr       string
	clients    chan *innerGRpcClient
	l          sync.Mutex
	tag        string
}

func NewInnerCon(addr string, maxClient, conTimeout int, tag string) *InnerCon {
	return &InnerCon{
		addr:       addr,
		maxClient:  maxClient,
		conTimeout: conTimeout,
		clients:    make(chan *innerGRpcClient, maxClient),
		tag:        tag,
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
	defer atomic.AddInt32(&th.curNum, 1)
	if len(th.clients) == 0 && int(th.allocNum) < th.maxClient {
		if idx := int(atomic.AddInt32(&th.allocNum, 1)); idx <= th.maxClient {
			common.Debugf("[%s]Get,New,%d", th.addr, atomic.LoadInt32(&th.allocNum))
			c := NewInnerGRpcClient(th.conTimeout)
			if err := c.Connect(th.addr, th.tag); err == nil {
				c.Idx = idx
				th.clients <- c
			}
		} else {
			atomic.AddInt32(&th.allocNum, -1)
		}
	}
	return <-th.clients
}
func (th *InnerCon) BackTo(c *innerGRpcClient) {
	if c == nil {
		return
	}
	defer atomic.AddInt32(&th.curNum, -1)
	//logrus.Debugf("[%s]BackTo,%s,%d", th.tag, th.addr, c.Idx)
	//defer logrus.Debugf("[%s]BackTo end,%s,%d", th.tag, th.addr, c.Idx)
	th.clients <- c
}
func (th *InnerCon) Close() {
	//分配的全部释放
	for atomic.LoadInt32(&th.allocNum) > 0 {
		c := <-th.clients
		c.Close()
		atomic.AddInt32(&th.allocNum, -1)
	}
}
