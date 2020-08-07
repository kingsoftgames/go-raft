package app

import (
	"context"
	"fmt"

	"git.shiyou.kingsoft.com/WANGXU13/ppx-app/inner"
)

type RaftServerGrpc struct {
	inner.UnimplementedRaftServer
	App *MainApp
}

func (th *RaftServerGrpc) JoinRequest(ctx context.Context, req *inner.JoinReq) (*inner.JoinRsp, error) {
	fmt.Printf("[API]JoinRequest,%s,%s\n", req.NodeId, req.Addr)
	rsp := &inner.JoinRsp{}
	if th.App.GetStore().IsFollower() {
		return th.App.service.GetInner().JoinRequest(ctx, req)
	}
	if err := th.App.GetStore().Join(req.NodeId, req.Addr, req.ApiAddr); err != nil {
		rsp.Result = -1
		rsp.Message = err.Error()
		return rsp, err
	}
	return rsp, nil
}
func (th *RaftServerGrpc) TransHttpRequest(ctx context.Context, req *inner.TransHttpReq) (*inner.TransHttpRsp, error) {
	rsp := &inner.TransHttpRsp{}
	if len(req.Hash) != 0 {
		ctx = context.WithValue(ctx, "hash", req.Hash)
	}
	if data, err := th.App.HttpCall(ctx, req.Path, req.Data); err == nil {
		rsp.Data = data
		return rsp, err
	}
	return rsp, nil
}
