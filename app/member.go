package app

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"
)

type Member struct {
	NodeId         string
	RaftAddr       string
	InnerAddr      string
	Ver            string
	State          int
	Con            *InnerCon
	LastHealthTime int64
	OverTimeCnt    int32
	LastIndex      uint64
}

func (th *Member) Health(health bool) {
	if health {
		atomic.StoreInt64(&th.LastHealthTime, time.Now().Unix())
		atomic.StoreInt32(&th.OverTimeCnt, 0)
	} else {
		atomic.AddInt32(&th.OverTimeCnt, 1)
	}
}

type MemberList struct {
	l          sync.RWMutex
	selfNodeId string

	mem          map[string]*Member
	raftAddrMap  map[string]string
	innerAddrMap map[string]string

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
func (th *MemberList) GetByRaftAddr(addr string) *Member {
	th.l.RLock()
	defer th.l.RUnlock()
	if th.raftAddrMap == nil {
		return nil
	}
	if id, ok := th.raftAddrMap[addr]; ok {
		return th.mem[id]
	}
	return nil
}
func (th *MemberList) GetByGrpcAddr(addr string) *Member {
	th.l.RLock()
	defer th.l.RUnlock()
	if th.innerAddrMap == nil {
		return nil
	}
	if id, ok := th.innerAddrMap[addr]; ok {
		return th.mem[id]
	}
	return nil
}
func (th *MemberList) get(nodeId string) *Member {
	if th.mem == nil {
		return nil
	}
	return th.mem[nodeId]
}
func (th *MemberList) Get(nodeId string) *Member {
	th.l.RLock()
	defer th.l.RUnlock()
	return th.get(nodeId)
}
func (th *MemberList) Add(m *Member) error {
	th.l.Lock()
	defer th.l.Unlock()
	if th.mem == nil {
		th.mem = map[string]*Member{}
		th.raftAddrMap = map[string]string{}
		th.innerAddrMap = map[string]string{}
	}
	if _m, ok := th.mem[m.NodeId]; ok {
		_m.LastIndex = m.LastIndex
		//if m.OverTimeCnt > 0 {
		//	if err := m.Con.ReConnect(m.GrpcAddr); err != nil {
		//		return err
		//	}
		//}
		return nil
	}
	if th.selfNodeId != m.NodeId { //not need connect self node
		m.Con = NewInnerCon(m.InnerAddr, poolMaxConnect, grpcTimeoutMs, th.selfNodeId)
		//if err := m.Con.Connect(m.GrpcAddr); err != nil {
		//	return err
		//}
		//th.OnConEvent.Emit(m)
	}
	th.mem[m.NodeId] = m
	th.raftAddrMap[m.RaftAddr] = m.NodeId
	th.innerAddrMap[m.InnerAddr] = m.NodeId
	logrus.Infof("[%s]MemberList.Add,%s,%s", th.selfNodeId, m.NodeId, m.InnerAddr)
	return nil
}
func (th *MemberList) Remove(nodeId string) bool {
	th.l.Lock()
	defer th.l.Unlock()
	m := th.get(nodeId)
	if m == nil {
		return false
	}
	if m.Con != nil {
		m.Con.Close()
	}
	delete(th.mem, nodeId)
	delete(th.raftAddrMap, m.RaftAddr)
	delete(th.innerAddrMap, m.InnerAddr)
	return true
}
func (th *MemberList) LeaveToAll(nodeId string) {
	th.l.RLock()
	defer th.l.RUnlock()
	for _, m := range th.mem {
		if th.selfNodeId == m.NodeId {
			continue
		}
		m.Con.GetRaftClient(func(client inner.RaftClient) {
			if client != nil {
				ctx, _ := context.WithTimeout(context.Background(), grpcTimeoutMs*time.Millisecond)
				msg := &inner.RemoveMemberReq{
					NodeId: nodeId,
				}
				rsp, err := client.RemoveMember(ctx, msg)
				if err != nil {
					logrus.Errorf("[%s]MemberList,RemoveMember,error,%s,%s,%s", th.selfNodeId, m.NodeId, nodeId, err.Error())
				} else if rsp.Result != 0 {
					logrus.Errorf("[%s]MemberList,RemoveMember,failed,%s,%s,%d", th.selfNodeId, m.NodeId, nodeId, rsp.Result)
				} else {
					logrus.Infof("[%s]MemberList,RemoveMember,ok,%s,%s", th.selfNodeId, m.NodeId, nodeId)
				}
			}
		})
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
			NodeId:    member.NodeId,
			InnerAddr: member.InnerAddr,
			RaftAddr:  member.RaftAddr,
			Ver:       member.Ver,
			LastIndex: member.LastIndex,
		})
	})
	th.l.RLock()
	defer th.l.RUnlock()
	for _, m := range th.mem {
		if th.selfNodeId == m.NodeId {
			continue
		}
		m.Con.GetRaftClient(func(client inner.RaftClient) {
			if client != nil {
				rsp, err := client.SynMember(ctx, msg)
				if err != nil {
					logrus.Errorf("[%s]MemberList,Syn,error,%s,%s", th.selfNodeId, m.NodeId, err.Error())
				} else if rsp.Result != 0 {
					logrus.Errorf("[%s]MemberList,Syn,failed,%s,%d", th.selfNodeId, m.NodeId, rsp.Result)
				}
			}
		})
	}
	return nil
}
