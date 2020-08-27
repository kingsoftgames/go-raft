// Code generated by protoc-gen-ppx. DO NOT EDIT.

package test

import (
	context "context"

	app "git.shiyou.kingsoft.com/infra/go-raft/app"
)

// ImplementedTestServer  with app.
type ImplementedTestServer struct {
	UnimplementedTestServer
	app app.IApp
}

func (th *ImplementedTestServer) GetRequest(ctx context.Context, req *GetReq) (*GetRsp, error) {
	f := app.NewReplyFuture(context.WithValue(ctx, "hash", req.Header.Hash), req, &GetRsp{})
	th.app.GRpcHandle(f)
	if f.Error() != nil {
		return nil, f.Error()
	}
	return f.Response().(*GetRsp), nil
}
func (th *ImplementedTestServer) SetRequest(ctx context.Context, req *SetReq) (*SetRsp, error) {
	f := app.NewReplyFuture(context.WithValue(ctx, "hash", req.Header.Hash), req, &SetRsp{})
	th.app.GRpcHandle(f)
	if f.Error() != nil {
		return nil, f.Error()
	}
	return f.Response().(*SetRsp), nil
}
func (th *ImplementedTestServer) DelRequest(ctx context.Context, req *DelReq) (*DelRsp, error) {
	f := app.NewReplyFuture(context.WithValue(ctx, "hash", req.Header.Hash), req, &DelRsp{})
	th.app.GRpcHandle(f)
	if f.Error() != nil {
		return nil, f.Error()
	}
	return f.Response().(*DelRsp), nil
}
