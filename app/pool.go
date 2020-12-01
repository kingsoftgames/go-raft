package app

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"

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
	conTimeout time.Duration
	addr       string
	clients    chan *innerGRpcClient
	l          sync.Mutex
	tag        string
	closeLock  sync.Mutex
}

func NewInnerCon(addr string, maxClient int, conTimeout time.Duration, tag string) *InnerCon {
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
	th.closeLock.Lock()
	defer th.closeLock.Unlock()
	if th.clients == nil {
		return nil
	}
	//f := common.GetFrame(2)
	//common.Debugf("[%s]Get %s:%d", th.tag, f.File, f.Line)
	common.Debugf("[%s]Get,%s", th.tag, th.addr)
	defer atomic.AddInt32(&th.curNum, 1)
	if len(th.clients) == 0 && int(atomic.LoadInt32(&th.allocNum)) < th.maxClient {
		if idx := int(atomic.AddInt32(&th.allocNum, 1)); idx <= th.maxClient {
			common.Debugf("[%s]Get,New,%s,%d", th.tag, th.addr, idx)
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
	th.closeLock.Lock()
	defer th.closeLock.Unlock()
	if th.clients == nil {
		c.Close()
		atomic.AddInt32(&th.allocNum, -1)
		atomic.AddInt32(&th.curNum, -1)
		logrus.Infof("[%s]BackTo finished,%d,%d", th.tag, atomic.LoadInt32(&th.allocNum), atomic.LoadInt32(&th.curNum))
		return
	}
	defer atomic.AddInt32(&th.curNum, -1)
	common.Debugf("[%s]BackTo,%s,%d", th.tag, th.addr, c.Idx)
	th.clients <- c
}
func (th *InnerCon) Close() {
	logrus.Infof("[%s]Close,%s,%d,%d,%d", th.tag, th.addr, atomic.LoadInt32(&th.allocNum), atomic.LoadInt32(&th.curNum), len(th.clients))
	defer logrus.Infof("[%s]Close finished,%s,%d,%d,%d", th.tag, th.addr, atomic.LoadInt32(&th.allocNum), atomic.LoadInt32(&th.curNum), len(th.clients))
	th.closeLock.Lock()
	defer func() {
		close(th.clients)
		th.clients = nil
		th.closeLock.Unlock()
	}()
	//分配的全部释放
	for {
		select {
		case c := <-th.clients:
			c.Close()
			atomic.AddInt32(&th.allocNum, -1)
		default:
			return
		}
	}
}
