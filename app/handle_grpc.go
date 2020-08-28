package app

import (
	"context"
	"time"

	"git.shiyou.kingsoft.com/infra/go-raft/common"

	"github.com/sirupsen/logrus"

	"git.shiyou.kingsoft.com/infra/go-raft/inner"
)

type RaftServerGRpc struct {
	inner.UnimplementedRaftServer
	App *MainApp
}

func (th *RaftServerGRpc) JoinRequest(ctx context.Context, req *inner.JoinReq) (*inner.JoinRsp, error) {
	logrus.Infof("[API][%s]JoinRequest,%s,%s", th.App.config.NodeId, req.NodeId, req.Addr)
	rsp := &inner.JoinRsp{}
	if err := th.App.Join(req.NodeId, req.Addr, req.ApiAddr); err != nil {
		rsp.Result = -1
		rsp.Message = err.Error()
		return rsp, err
	}
	return rsp, nil
}
func (th *RaftServerGRpc) SynMember(ctx context.Context, req *inner.SynMemberReq) (*inner.SynMemberRsp, error) {
	logrus.Infof("[API][%s]SynMember,%v", th.App.config.NodeId, req.Mem)
	rsp := &inner.SynMemberRsp{}
	th.App.config.Bootstrap = req.Bootstrap
	th.App.config.BootstrapExpect = int(req.BootstrapExpect)
	for _, m := range req.Mem {
		mem := th.App.members.Get(m.NodeId)
		if mem != nil {
			continue
		}
		if err := th.App.members.Add(&Member{
			NodeId:   m.NodeId,
			RaftAddr: m.RaftAddr,
			GrpcAddr: m.GrpcAddr,
		}); err != nil {
			logrus.Errorf("[API]SynMember,Add,error,%s,%s", m.NodeId, err.Error())
		}
	}
	return rsp, nil
}
func (th *RaftServerGRpc) TransHttpRequest(ctx context.Context, req *inner.TransHttpReq) (*inner.TransHttpRsp, error) {
	rsp := &inner.TransHttpRsp{}
	if len(req.Hash) != 0 {
		ctx = context.WithValue(ctx, "hash", req.Hash)
	}
	data, err := th.App.HttpCall(ctx, req.Path, req.Data)
	if err == nil {
		rsp.Data = data
	}
	return rsp, err
}

func (th *RaftServerGRpc) TransGrpcRequest(ctx context.Context, req *inner.TransGrpcReq) (*inner.TransGrpcRsp, error) {
	//logrus.Debugf("[API]TransGrpcRequest,%s,%s", req.Hash, req.Name)
	rsp := &inner.TransGrpcRsp{}
	context.WithValue(ctx, "hash", req.Hash)
	data, err := th.App.GRpcCall(ctx, req.Name, req.Data)
	if err == nil {
		rsp.Data = data
	}
	return rsp, err
}
func (th *RaftServerGRpc) HealthRequest(ctx context.Context, req *inner.HealthReq) (*inner.HealthRsp, error) {
	common.Debugf("[%s]HealthRequest from [%s],%d", th.App.config.NodeId, req.NodeId, (time.Now().UnixNano()-req.SendTime)/1e6)
	if err := th.App.OnlyJoin(req.NodeId, req.Addr, req.ApiAddr); err != nil {
		logrus.Errorf("[%s]HealthRequest,%v", th.App.config.NodeId, *req)
	}
	return &inner.HealthRsp{
		RecvTime: req.SendTime,
		SendTime: time.Now().UnixNano(),
	}, nil
}
