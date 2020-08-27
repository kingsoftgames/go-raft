package app

import (
	"context"
	"sync"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"
)

type Member struct {
	NodeId         string
	RaftAddr       string
	GrpcAddr       string
	State          int
	Con            *innerGRpcClient
	LastHealthTime int64
	OverTimeCnt    int
}

func (th *Member) Health(health bool) {
	if health {
		th.LastHealthTime = time.Now().Unix()
		th.OverTimeCnt = 0
	} else {
		th.OverTimeCnt++
	}
}

type MemberList struct {
	l          sync.RWMutex
	selfNodeId string
	mem        map[string]*Member
	addrMap    map[string]string

	OnConEvent common.SafeEvent
}

func (th *MemberList) Len() int {
	th.l.RLock()
	defer th.l.RUnlock()
	return len(th.mem)
}
func (th *MemberList) Foreach(cb func(member *Member)) {
	th.l.RLock()
	defer th.l.RUnlock()
	for _, m := range th.mem {
		cb(m)
	}
}
func (th *MemberList) GetByAddr(addr string) *Member {
	if th.addrMap == nil {
		return nil
	}
	th.l.RLock()
	defer th.l.RUnlock()
	if id, ok := th.addrMap[addr]; ok {
		return th.mem[id]
	}
	return nil
}
func (th *MemberList) Get(nodeId string) *Member {
	if th.mem == nil {
		return nil
	}
	th.l.RLock()
	defer th.l.RUnlock()
	return th.mem[nodeId]
}
func (th *MemberList) Add(m *Member) error {
	th.l.Lock()
	defer th.l.Unlock()
	if th.mem == nil {
		th.mem = map[string]*Member{}
		th.addrMap = map[string]string{}
	}
	if _, ok := th.mem[m.NodeId]; ok {
		return nil
	}
	if th.selfNodeId != m.NodeId { //not need connect self node
		m.Con = NewInnerGRpcClient(2000)
		if err := m.Con.Connect(m.GrpcAddr); err != nil {
			return err
		}
		th.OnConEvent.Emit(m)
	}
	th.mem[m.NodeId] = m
	th.addrMap[m.RaftAddr] = m.NodeId
	return nil
}
func (th *MemberList) Remove(nodeId string) {
	m := th.Get(nodeId)
	if m != nil {
		m.Con.Stop()
		th.l.Lock()
		defer th.l.Unlock()
		delete(th.mem, nodeId)
		delete(th.addrMap, m.RaftAddr)
	}
}
func (th *MemberList) SynMemberToAll(bootstrap bool, bootstrapExpect int) error {
	ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
	msg := &inner.SynMemberReq{
		Bootstrap:       bootstrap,
		BootstrapExpect: int32(bootstrapExpect),
		Mem:             make([]*inner.Member, 0),
	}
	th.Foreach(func(member *Member) {
		msg.Mem = append(msg.Mem, &inner.Member{
			NodeId:   member.NodeId,
			GrpcAddr: member.GrpcAddr,
			RaftAddr: member.RaftAddr,
		})
	})
	th.l.RLock()
	defer th.l.RUnlock()
	for _, m := range th.mem {
		if th.selfNodeId == m.NodeId {
			continue
		}
		rsp, err := m.Con.GetClient().SynMember(ctx, msg)
		if err != nil {
			logrus.Errorf("MemberList,Syn,error,%s,%s", m.NodeId, err.Error())
			continue
		}
		if rsp.Result != 0 {
			logrus.Errorf("MemberList,Syn,failed,%s,%d", m.NodeId, rsp.Result)
		}
	}
	return nil
}
