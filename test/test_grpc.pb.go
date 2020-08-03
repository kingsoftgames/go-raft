// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package test

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// TestClient is the client API for Test service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type TestClient interface {
	GetRequest(ctx context.Context, in *GetReq, opts ...grpc.CallOption) (*GetRsp, error)
	SetRequest(ctx context.Context, in *SetReq, opts ...grpc.CallOption) (*SetRsp, error)
	DelRequest(ctx context.Context, in *DelReq, opts ...grpc.CallOption) (*DelRsp, error)
}

type testClient struct {
	cc grpc.ClientConnInterface
}

func NewTestClient(cc grpc.ClientConnInterface) TestClient {
	return &testClient{cc}
}

func (c *testClient) GetRequest(ctx context.Context, in *GetReq, opts ...grpc.CallOption) (*GetRsp, error) {
	out := new(GetRsp)
	err := c.cc.Invoke(ctx, "/test.Test/GetRequest", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testClient) SetRequest(ctx context.Context, in *SetReq, opts ...grpc.CallOption) (*SetRsp, error) {
	out := new(SetRsp)
	err := c.cc.Invoke(ctx, "/test.Test/SetRequest", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *testClient) DelRequest(ctx context.Context, in *DelReq, opts ...grpc.CallOption) (*DelRsp, error) {
	out := new(DelRsp)
	err := c.cc.Invoke(ctx, "/test.Test/DelRequest", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TestServer is the server API for Test service.
// All implementations must embed UnimplementedTestServer
// for forward compatibility
type TestServer interface {
	GetRequest(context.Context, *GetReq) (*GetRsp, error)
	SetRequest(context.Context, *SetReq) (*SetRsp, error)
	DelRequest(context.Context, *DelReq) (*DelRsp, error)
	mustEmbedUnimplementedTestServer()
}

// UnimplementedTestServer must be embedded to have forward compatible implementations.
type UnimplementedTestServer struct {
}

func (*UnimplementedTestServer) GetRequest(context.Context, *GetReq) (*GetRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetRequest not implemented")
}
func (*UnimplementedTestServer) SetRequest(context.Context, *SetReq) (*SetRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetRequest not implemented")
}
func (*UnimplementedTestServer) DelRequest(context.Context, *DelReq) (*DelRsp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DelRequest not implemented")
}
func (*UnimplementedTestServer) mustEmbedUnimplementedTestServer() {}

func RegisterTestServer(s *grpc.Server, srv TestServer) {
	s.RegisterService(&_Test_serviceDesc, srv)
}

func _Test_GetRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServer).GetRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Test/GetRequest",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServer).GetRequest(ctx, req.(*GetReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Test_SetRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServer).SetRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Test/SetRequest",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServer).SetRequest(ctx, req.(*SetReq))
	}
	return interceptor(ctx, in, info, handler)
}

func _Test_DelRequest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DelReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TestServer).DelRequest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/test.Test/DelRequest",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TestServer).DelRequest(ctx, req.(*DelReq))
	}
	return interceptor(ctx, in, info, handler)
}

var _Test_serviceDesc = grpc.ServiceDesc{
	ServiceName: "test.Test",
	HandlerType: (*TestServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetRequest",
			Handler:    _Test_GetRequest_Handler,
		},
		{
			MethodName: "SetRequest",
			Handler:    _Test_SetRequest_Handler,
		},
		{
			MethodName: "DelRequest",
			Handler:    _Test_DelRequest_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "test.proto",
}
