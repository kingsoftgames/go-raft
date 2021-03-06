package app

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/raft"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"
)

type RaftServerGRpc struct {
	inner.UnimplementedRaftServer
	App *MainApp
}

func (th *RaftServerGRpc) JoinRequest(ctx context.Context, req *inner.JoinReq) (*inner.JoinRsp, error) {
	logrus.Infof("[API][%s]JoinRequest,%v", th.App.getNodeName(), *req.Info)
	defer logrus.Debugf("[API][%s]JoinRequest finished,%v", th.App.getNodeName(), *req.Info)
	f := NewReplyFuturePrioritized(ctx, req, &inner.JoinRsp{})
	defer PutReplyFuture(f)
	f.cmd = FutureCmdTypeJoin
	th.App.GRpcHandle(f)
	if f.Error() != nil {
		return nil, f.Error()
	}
	return f.Response().(*inner.JoinRsp), nil
}
func (th *RaftServerGRpc) SynMember(ctx context.Context, req *inner.SynMemberReq) (*inner.SynMemberRsp, error) {
	logrus.Infof("[API][%s]SynMember,%s,%v", th.App.getNodeName(), req.NodeId, req.Mem)
	defer logrus.Infof("[API][%s]SynMember finished ,%s,%v", th.App.getNodeName(), req.NodeId, req.Mem)
	f := NewReplyFuturePrioritized(ctx, req, &inner.SynMemberRsp{})
	defer PutReplyFuture(f)
	f.cmd = FutureCmdTypeSynMember
	th.App.GRpcHandle(f)
	if f.Error() != nil {
		return nil, f.Error()
	}
	return f.Response().(*inner.SynMemberRsp), nil
}
func (th *RaftServerGRpc) TransHttpRequest(ctx context.Context, req *inner.TransHttpReq) (*inner.TransHttpRsp, error) {
	rsp := &inner.TransHttpRsp{}
	if len(req.Hash) != 0 {
		ctx = context.WithValue(ctx, "hash", req.Hash)
	}
	data, err := th.App.HttpCall(ctx, strings.ReplaceAll(req.Path, "/", "."), req.Data)
	if err == nil {
		rsp.Data = data
	}
	return rsp, err
}

func (th *RaftServerGRpc) TransGrpcRequest(ctx context.Context, req *inner.TransGrpcReq) (rsp *inner.TransGrpcRsp, err error) {
	if req.Prioritized {
		logrus.Debugf("[%s][API]TransGrpcRequest,%s,%s", th.App.getNodeName(), req.Hash, req.Name)
		th.App.debug.Store(1)
	}
	hd := th.App.handler.GetHandlerValue(req.Name)
	if hd == nil {
		return nil, fmt.Errorf("not found method,%s", req.Name)
	}
	_req := reflect.New(hd.paramReq)
	if err = common.Decode(req.Data, _req.Interface()); err != nil {
		return
	}
	tt := time.Now()
	defer logrus.Debugf("TransGrpcRequest rsp %d", time.Now().Sub(tt).Microseconds())
	_rsp := reflect.New(hd.paramRsp)
	if len(req.Hash) > 0 {
		ctx = context.WithValue(ctx, "hash", req.Hash)
	}
	f := NewReplyFuture(ctx, _req.Interface(), _rsp.Interface())
	f.prioritized = req.Prioritized
	f.trans = true
	f.cmd = FutureCmdTypeGRpc
	for _, line := range req.Timeline {
		f.AddTimelineObj(TimelineInfo{
			Tag: line.Tag,
			T:   line.T,
		})
	}
	f.AddTimeLine("TransGrpcRequest")
	th.App.GRpcHandle(f)
	if f.Error() != nil && f.Error() != errTransfer {
		return nil, f.Error()
	}
	f.AddTimeLine("TransGrpcRequestResult")
	rsp = &inner.TransGrpcRsp{
		Timeline: make([]*inner.TimeLineUnit, 0),
	}
	for _, line := range f.Timeline {
		rsp.Timeline = append(rsp.Timeline, &inner.TimeLineUnit{
			Tag: line.Tag,
			T:   line.T,
		})
	}
	if f.Error() == errTransfer {
		rsp.Back = true
	} else {
		rsp.Data, err = common.Encode(_rsp.Interface())
	}
	return
}
func (th *RaftServerGRpc) HealthRequest(_ context.Context, req *inner.HealthReq) (*inner.HealthRsp, error) {
	common.Debugf("[%s]HealthRequest from [%s],%d,%d", th.App.getNodeName(), req.Info.NodeId, req.Info.LastIndex, (time.Now().UnixNano()-req.SendTime)/1e6)
	if err := th.App.OnlyJoin(req.Info); err != nil {
		logrus.Errorf("[%s]HealthRequest,%v", th.App.getNodeName(), *req)
	}
	return &inner.HealthRsp{
		RecvTime: req.SendTime,
		SendTime: time.Now().UnixNano(),
	}, nil
}

func (th *RaftServerGRpc) RemoveMember(ctx context.Context, req *inner.RemoveMemberReq) (*inner.RemoveMemberRsp, error) {
	logrus.Infof("[API][%s]RemoveMember,%s", th.App.getNodeName(), req.NodeId)
	//defer logrus.Debugf("[API][%s]RemoveMember finished,%s", th.App.getNodeName(), req.NodeId)
	f := NewReplyFuturePrioritized(ctx, req, &inner.RemoveMemberRsp{})
	defer PutReplyFuture(f)
	f.cmd = FutureCmdTypeRemove
	th.App.GRpcHandle(f)
	if f.Error() != nil {
		return nil, f.Error()
	}
	return f.Response().(*inner.RemoveMemberRsp), nil
}
func (th *RaftServerGRpc) ExitRequest(ctx context.Context, req *inner.ExitReq) (*inner.ExitRsp, error) {
	logrus.Infof("[API][%s]ExitRequest,%s", th.App.getNodeName(), req.NodeId)
	rsp := &inner.ExitRsp{}
	f := NewReplyFuturePrioritized(ctx, req, rsp)
	defer PutReplyFuture(f)
	f.cmd = FutureCmdTypeExit
	th.App.GRpcHandle(f)
	if f.Error() != nil && f.Error() != raft.ErrNotLeader {
		return nil, f.Error()
	}
	if f.Error() == raft.ErrNotLeader {
		rsp.Result = ResultCodeNotLeader
	}
	return f.Response().(*inner.ExitRsp), nil
}
