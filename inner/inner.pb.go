// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.12.3
// source: inner.proto

package inner

import (
	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type ApplyCmd_CmdType int32

const (
	ApplyCmd_INVALID ApplyCmd_CmdType = 0
	ApplyCmd_GET     ApplyCmd_CmdType = 1
	ApplyCmd_SET     ApplyCmd_CmdType = 2
	ApplyCmd_DEL     ApplyCmd_CmdType = 3
)

// Enum value maps for ApplyCmd_CmdType.
var (
	ApplyCmd_CmdType_name = map[int32]string{
		0: "INVALID",
		1: "GET",
		2: "SET",
		3: "DEL",
	}
	ApplyCmd_CmdType_value = map[string]int32{
		"INVALID": 0,
		"GET":     1,
		"SET":     2,
		"DEL":     3,
	}
)

func (x ApplyCmd_CmdType) Enum() *ApplyCmd_CmdType {
	p := new(ApplyCmd_CmdType)
	*p = x
	return p
}

func (x ApplyCmd_CmdType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ApplyCmd_CmdType) Descriptor() protoreflect.EnumDescriptor {
	return file_inner_proto_enumTypes[0].Descriptor()
}

func (ApplyCmd_CmdType) Type() protoreflect.EnumType {
	return &file_inner_proto_enumTypes[0]
}

func (x ApplyCmd_CmdType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ApplyCmd_CmdType.Descriptor instead.
func (ApplyCmd_CmdType) EnumDescriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{11, 0}
}

type JoinReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NodeId  string `protobuf:"bytes,1,opt,name=nodeId,proto3" json:"nodeId,omitempty"`   //cur node Id
	Addr    string `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`       //cur node raft bind addr
	ApiAddr string `protobuf:"bytes,3,opt,name=apiAddr,proto3" json:"apiAddr,omitempty"` //cur node api addr
}

func (x *JoinReq) Reset() {
	*x = JoinReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JoinReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JoinReq) ProtoMessage() {}

func (x *JoinReq) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JoinReq.ProtoReflect.Descriptor instead.
func (*JoinReq) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{0}
}

func (x *JoinReq) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *JoinReq) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *JoinReq) GetApiAddr() string {
	if x != nil {
		return x.ApiAddr
	}
	return ""
}

type JoinRsp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result  int32  `protobuf:"varint,1,opt,name=result,proto3" json:"result,omitempty"` //0 ok
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *JoinRsp) Reset() {
	*x = JoinRsp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *JoinRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*JoinRsp) ProtoMessage() {}

func (x *JoinRsp) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use JoinRsp.ProtoReflect.Descriptor instead.
func (*JoinRsp) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{1}
}

func (x *JoinRsp) GetResult() int32 {
	if x != nil {
		return x.Result
	}
	return 0
}

func (x *JoinRsp) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

type Member struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NodeId   string `protobuf:"bytes,1,opt,name=nodeId,proto3" json:"nodeId,omitempty"`
	RaftAddr string `protobuf:"bytes,2,opt,name=raftAddr,proto3" json:"raftAddr,omitempty"`
	GrpcAddr string `protobuf:"bytes,3,opt,name=grpcAddr,proto3" json:"grpcAddr,omitempty"`
}

func (x *Member) Reset() {
	*x = Member{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Member) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Member) ProtoMessage() {}

func (x *Member) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Member.ProtoReflect.Descriptor instead.
func (*Member) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{2}
}

func (x *Member) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *Member) GetRaftAddr() string {
	if x != nil {
		return x.RaftAddr
	}
	return ""
}

func (x *Member) GetGrpcAddr() string {
	if x != nil {
		return x.GrpcAddr
	}
	return ""
}

type SynMemberReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Mem             []*Member `protobuf:"bytes,1,rep,name=mem,proto3" json:"mem,omitempty"`
	Bootstrap       bool      `protobuf:"varint,2,opt,name=bootstrap,proto3" json:"bootstrap,omitempty"`
	BootstrapExpect int32     `protobuf:"varint,3,opt,name=bootstrapExpect,proto3" json:"bootstrapExpect,omitempty"`
}

func (x *SynMemberReq) Reset() {
	*x = SynMemberReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SynMemberReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SynMemberReq) ProtoMessage() {}

func (x *SynMemberReq) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SynMemberReq.ProtoReflect.Descriptor instead.
func (*SynMemberReq) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{3}
}

func (x *SynMemberReq) GetMem() []*Member {
	if x != nil {
		return x.Mem
	}
	return nil
}

func (x *SynMemberReq) GetBootstrap() bool {
	if x != nil {
		return x.Bootstrap
	}
	return false
}

func (x *SynMemberReq) GetBootstrapExpect() int32 {
	if x != nil {
		return x.BootstrapExpect
	}
	return 0
}

type SynMemberRsp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result int32 `protobuf:"varint,1,opt,name=result,proto3" json:"result,omitempty"`
}

func (x *SynMemberRsp) Reset() {
	*x = SynMemberRsp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SynMemberRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SynMemberRsp) ProtoMessage() {}

func (x *SynMemberRsp) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SynMemberRsp.ProtoReflect.Descriptor instead.
func (*SynMemberRsp) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{4}
}

func (x *SynMemberRsp) GetResult() int32 {
	if x != nil {
		return x.Result
	}
	return 0
}

type TransHttpReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash string `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Path string `protobuf:"bytes,2,opt,name=path,proto3" json:"path,omitempty"`
	Data []byte `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *TransHttpReq) Reset() {
	*x = TransHttpReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TransHttpReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransHttpReq) ProtoMessage() {}

func (x *TransHttpReq) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransHttpReq.ProtoReflect.Descriptor instead.
func (*TransHttpReq) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{5}
}

func (x *TransHttpReq) GetHash() string {
	if x != nil {
		return x.Hash
	}
	return ""
}

func (x *TransHttpReq) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *TransHttpReq) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type TransHttpRsp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *TransHttpRsp) Reset() {
	*x = TransHttpRsp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TransHttpRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransHttpRsp) ProtoMessage() {}

func (x *TransHttpRsp) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransHttpRsp.ProtoReflect.Descriptor instead.
func (*TransHttpRsp) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{6}
}

func (x *TransHttpRsp) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type TransGrpcReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Hash string `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	Data []byte `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *TransGrpcReq) Reset() {
	*x = TransGrpcReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TransGrpcReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransGrpcReq) ProtoMessage() {}

func (x *TransGrpcReq) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransGrpcReq.ProtoReflect.Descriptor instead.
func (*TransGrpcReq) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{7}
}

func (x *TransGrpcReq) GetHash() string {
	if x != nil {
		return x.Hash
	}
	return ""
}

func (x *TransGrpcReq) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *TransGrpcReq) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type TransGrpcRsp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Data []byte `protobuf:"bytes,1,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *TransGrpcRsp) Reset() {
	*x = TransGrpcRsp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TransGrpcRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TransGrpcRsp) ProtoMessage() {}

func (x *TransGrpcRsp) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TransGrpcRsp.ProtoReflect.Descriptor instead.
func (*TransGrpcRsp) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{8}
}

func (x *TransGrpcRsp) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

type HealthReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	NodeId  string `protobuf:"bytes,1,opt,name=nodeId,proto3" json:"nodeId,omitempty"`   //cur node Id
	Addr    string `protobuf:"bytes,2,opt,name=addr,proto3" json:"addr,omitempty"`       //cur node raft bind addr
	ApiAddr string `protobuf:"bytes,3,opt,name=apiAddr,proto3" json:"apiAddr,omitempty"` //cur node api addr
}

func (x *HealthReq) Reset() {
	*x = HealthReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HealthReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthReq) ProtoMessage() {}

func (x *HealthReq) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthReq.ProtoReflect.Descriptor instead.
func (*HealthReq) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{9}
}

func (x *HealthReq) GetNodeId() string {
	if x != nil {
		return x.NodeId
	}
	return ""
}

func (x *HealthReq) GetAddr() string {
	if x != nil {
		return x.Addr
	}
	return ""
}

func (x *HealthReq) GetApiAddr() string {
	if x != nil {
		return x.ApiAddr
	}
	return ""
}

type HealthRsp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *HealthRsp) Reset() {
	*x = HealthRsp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HealthRsp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HealthRsp) ProtoMessage() {}

func (x *HealthRsp) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HealthRsp.ProtoReflect.Descriptor instead.
func (*HealthRsp) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{10}
}

type ApplyCmd struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Cmd ApplyCmd_CmdType `protobuf:"varint,1,opt,name=cmd,proto3,enum=inner.ApplyCmd_CmdType" json:"cmd,omitempty"`
	// Types that are assignable to Obj:
	//	*ApplyCmd_Get
	//	*ApplyCmd_Set
	//	*ApplyCmd_Del
	Obj isApplyCmd_Obj `protobuf_oneof:"obj"`
}

func (x *ApplyCmd) Reset() {
	*x = ApplyCmd{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[11]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplyCmd) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyCmd) ProtoMessage() {}

func (x *ApplyCmd) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[11]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyCmd.ProtoReflect.Descriptor instead.
func (*ApplyCmd) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{11}
}

func (x *ApplyCmd) GetCmd() ApplyCmd_CmdType {
	if x != nil {
		return x.Cmd
	}
	return ApplyCmd_INVALID
}

func (m *ApplyCmd) GetObj() isApplyCmd_Obj {
	if m != nil {
		return m.Obj
	}
	return nil
}

func (x *ApplyCmd) GetGet() *ApplyCmd_OpGet {
	if x, ok := x.GetObj().(*ApplyCmd_Get); ok {
		return x.Get
	}
	return nil
}

func (x *ApplyCmd) GetSet() *ApplyCmd_OpSet {
	if x, ok := x.GetObj().(*ApplyCmd_Set); ok {
		return x.Set
	}
	return nil
}

func (x *ApplyCmd) GetDel() *ApplyCmd_OpDel {
	if x, ok := x.GetObj().(*ApplyCmd_Del); ok {
		return x.Del
	}
	return nil
}

type isApplyCmd_Obj interface {
	isApplyCmd_Obj()
}

type ApplyCmd_Get struct {
	Get *ApplyCmd_OpGet `protobuf:"bytes,2,opt,name=get,proto3,oneof"`
}

type ApplyCmd_Set struct {
	Set *ApplyCmd_OpSet `protobuf:"bytes,3,opt,name=set,proto3,oneof"`
}

type ApplyCmd_Del struct {
	Del *ApplyCmd_OpDel `protobuf:"bytes,4,opt,name=del,proto3,oneof"`
}

func (*ApplyCmd_Get) isApplyCmd_Obj() {}

func (*ApplyCmd_Set) isApplyCmd_Obj() {}

func (*ApplyCmd_Del) isApplyCmd_Obj() {}

type ApplyCmd_OpGet struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
}

func (x *ApplyCmd_OpGet) Reset() {
	*x = ApplyCmd_OpGet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[12]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplyCmd_OpGet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyCmd_OpGet) ProtoMessage() {}

func (x *ApplyCmd_OpGet) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[12]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyCmd_OpGet.ProtoReflect.Descriptor instead.
func (*ApplyCmd_OpGet) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{11, 0}
}

func (x *ApplyCmd_OpGet) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

type ApplyCmd_OpSet struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key   string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Value []byte `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *ApplyCmd_OpSet) Reset() {
	*x = ApplyCmd_OpSet{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[13]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplyCmd_OpSet) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyCmd_OpSet) ProtoMessage() {}

func (x *ApplyCmd_OpSet) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[13]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyCmd_OpSet.ProtoReflect.Descriptor instead.
func (*ApplyCmd_OpSet) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{11, 1}
}

func (x *ApplyCmd_OpSet) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *ApplyCmd_OpSet) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

type ApplyCmd_OpDel struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
}

func (x *ApplyCmd_OpDel) Reset() {
	*x = ApplyCmd_OpDel{}
	if protoimpl.UnsafeEnabled {
		mi := &file_inner_proto_msgTypes[14]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ApplyCmd_OpDel) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyCmd_OpDel) ProtoMessage() {}

func (x *ApplyCmd_OpDel) ProtoReflect() protoreflect.Message {
	mi := &file_inner_proto_msgTypes[14]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyCmd_OpDel.ProtoReflect.Descriptor instead.
func (*ApplyCmd_OpDel) Descriptor() ([]byte, []int) {
	return file_inner_proto_rawDescGZIP(), []int{11, 2}
}

func (x *ApplyCmd_OpDel) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

var File_inner_proto protoreflect.FileDescriptor

var file_inner_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x05, 0x69,
	0x6e, 0x6e, 0x65, 0x72, 0x22, 0x4f, 0x0a, 0x07, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x12,
	0x16, 0x0a, 0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x64, 0x64, 0x72, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x61,
	0x70, 0x69, 0x41, 0x64, 0x64, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x70,
	0x69, 0x41, 0x64, 0x64, 0x72, 0x22, 0x3b, 0x0a, 0x07, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x73, 0x70,
	0x12, 0x16, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x22, 0x58, 0x0a, 0x06, 0x4d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x16, 0x0a, 0x06,
	0x6e, 0x6f, 0x64, 0x65, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f,
	0x64, 0x65, 0x49, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x61, 0x66, 0x74, 0x41, 0x64, 0x64, 0x72,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x61, 0x66, 0x74, 0x41, 0x64, 0x64, 0x72,
	0x12, 0x1a, 0x0a, 0x08, 0x67, 0x72, 0x70, 0x63, 0x41, 0x64, 0x64, 0x72, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x67, 0x72, 0x70, 0x63, 0x41, 0x64, 0x64, 0x72, 0x22, 0x77, 0x0a, 0x0c,
	0x53, 0x79, 0x6e, 0x4d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x65, 0x71, 0x12, 0x1f, 0x0a, 0x03,
	0x6d, 0x65, 0x6d, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0d, 0x2e, 0x69, 0x6e, 0x6e, 0x65,
	0x72, 0x2e, 0x4d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x03, 0x6d, 0x65, 0x6d, 0x12, 0x1c, 0x0a,
	0x09, 0x62, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x09, 0x62, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x12, 0x28, 0x0a, 0x0f, 0x62,
	0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x45, 0x78, 0x70, 0x65, 0x63, 0x74, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x05, 0x52, 0x0f, 0x62, 0x6f, 0x6f, 0x74, 0x73, 0x74, 0x72, 0x61, 0x70, 0x45,
	0x78, 0x70, 0x65, 0x63, 0x74, 0x22, 0x26, 0x0a, 0x0c, 0x53, 0x79, 0x6e, 0x4d, 0x65, 0x6d, 0x62,
	0x65, 0x72, 0x52, 0x73, 0x70, 0x12, 0x16, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x22, 0x4a, 0x0a,
	0x0c, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x48, 0x74, 0x74, 0x70, 0x52, 0x65, 0x71, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x61, 0x73,
	0x68, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x22, 0x0a, 0x0c, 0x54, 0x72, 0x61,
	0x6e, 0x73, 0x48, 0x74, 0x74, 0x70, 0x52, 0x73, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x4a, 0x0a,
	0x0c, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x47, 0x72, 0x70, 0x63, 0x52, 0x65, 0x71, 0x12, 0x12, 0x0a,
	0x04, 0x68, 0x61, 0x73, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x61, 0x73,
	0x68, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x22, 0x0a, 0x0c, 0x54, 0x72, 0x61,
	0x6e, 0x73, 0x47, 0x72, 0x70, 0x63, 0x52, 0x73, 0x70, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22, 0x51, 0x0a,
	0x09, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x71, 0x12, 0x16, 0x0a, 0x06, 0x6e, 0x6f,
	0x64, 0x65, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6e, 0x6f, 0x64, 0x65,
	0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x64, 0x64, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x61, 0x64, 0x64, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x70, 0x69, 0x41, 0x64, 0x64,
	0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x70, 0x69, 0x41, 0x64, 0x64, 0x72,
	0x22, 0x0b, 0x0a, 0x09, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x73, 0x70, 0x22, 0xd7, 0x02,
	0x0a, 0x08, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x43, 0x6d, 0x64, 0x12, 0x29, 0x0a, 0x03, 0x63, 0x6d,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x17, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e,
	0x41, 0x70, 0x70, 0x6c, 0x79, 0x43, 0x6d, 0x64, 0x2e, 0x43, 0x6d, 0x64, 0x54, 0x79, 0x70, 0x65,
	0x52, 0x03, 0x63, 0x6d, 0x64, 0x12, 0x29, 0x0a, 0x03, 0x67, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x15, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x41, 0x70, 0x70, 0x6c, 0x79,
	0x43, 0x6d, 0x64, 0x2e, 0x4f, 0x70, 0x47, 0x65, 0x74, 0x48, 0x00, 0x52, 0x03, 0x67, 0x65, 0x74,
	0x12, 0x29, 0x0a, 0x03, 0x73, 0x65, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e,
	0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x43, 0x6d, 0x64, 0x2e, 0x4f,
	0x70, 0x53, 0x65, 0x74, 0x48, 0x00, 0x52, 0x03, 0x73, 0x65, 0x74, 0x12, 0x29, 0x0a, 0x03, 0x64,
	0x65, 0x6c, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72,
	0x2e, 0x41, 0x70, 0x70, 0x6c, 0x79, 0x43, 0x6d, 0x64, 0x2e, 0x4f, 0x70, 0x44, 0x65, 0x6c, 0x48,
	0x00, 0x52, 0x03, 0x64, 0x65, 0x6c, 0x1a, 0x19, 0x0a, 0x05, 0x4f, 0x70, 0x47, 0x65, 0x74, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x1a, 0x2f, 0x0a, 0x05, 0x4f, 0x70, 0x53, 0x65, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x1a, 0x19, 0x0a, 0x05, 0x4f, 0x70, 0x44, 0x65, 0x6c, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x22, 0x31, 0x0a,
	0x07, 0x43, 0x6d, 0x64, 0x54, 0x79, 0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x49, 0x4e, 0x56, 0x41,
	0x4c, 0x49, 0x44, 0x10, 0x00, 0x12, 0x07, 0x0a, 0x03, 0x47, 0x45, 0x54, 0x10, 0x01, 0x12, 0x07,
	0x0a, 0x03, 0x53, 0x45, 0x54, 0x10, 0x02, 0x12, 0x07, 0x0a, 0x03, 0x44, 0x45, 0x4c, 0x10, 0x03,
	0x42, 0x05, 0x0a, 0x03, 0x6f, 0x62, 0x6a, 0x32, 0xa7, 0x02, 0x0a, 0x04, 0x52, 0x61, 0x66, 0x74,
	0x12, 0x2f, 0x0a, 0x0b, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x0e, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x65, 0x71, 0x1a,
	0x0e, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x4a, 0x6f, 0x69, 0x6e, 0x52, 0x73, 0x70, 0x22,
	0x00, 0x12, 0x37, 0x0a, 0x09, 0x53, 0x79, 0x6e, 0x4d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x12, 0x13,
	0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x53, 0x79, 0x6e, 0x4d, 0x65, 0x6d, 0x62, 0x65, 0x72,
	0x52, 0x65, 0x71, 0x1a, 0x13, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x53, 0x79, 0x6e, 0x4d,
	0x65, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x73, 0x70, 0x22, 0x00, 0x12, 0x3e, 0x0a, 0x10, 0x54, 0x72,
	0x61, 0x6e, 0x73, 0x48, 0x74, 0x74, 0x70, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x13,
	0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x48, 0x74, 0x74, 0x70,
	0x52, 0x65, 0x71, 0x1a, 0x13, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x54, 0x72, 0x61, 0x6e,
	0x73, 0x48, 0x74, 0x74, 0x70, 0x52, 0x73, 0x70, 0x22, 0x00, 0x12, 0x3e, 0x0a, 0x10, 0x54, 0x72,
	0x61, 0x6e, 0x73, 0x47, 0x72, 0x70, 0x63, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x13,
	0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x54, 0x72, 0x61, 0x6e, 0x73, 0x47, 0x72, 0x70, 0x63,
	0x52, 0x65, 0x71, 0x1a, 0x13, 0x2e, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x54, 0x72, 0x61, 0x6e,
	0x73, 0x47, 0x72, 0x70, 0x63, 0x52, 0x73, 0x70, 0x22, 0x00, 0x12, 0x35, 0x0a, 0x0d, 0x48, 0x65,
	0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x2e, 0x69, 0x6e,
	0x6e, 0x65, 0x72, 0x2e, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x65, 0x71, 0x1a, 0x10, 0x2e,
	0x69, 0x6e, 0x6e, 0x65, 0x72, 0x2e, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x52, 0x73, 0x70, 0x22,
	0x00, 0x42, 0x09, 0x5a, 0x07, 0x2e, 0x3b, 0x69, 0x6e, 0x6e, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_inner_proto_rawDescOnce sync.Once
	file_inner_proto_rawDescData = file_inner_proto_rawDesc
)

func file_inner_proto_rawDescGZIP() []byte {
	file_inner_proto_rawDescOnce.Do(func() {
		file_inner_proto_rawDescData = protoimpl.X.CompressGZIP(file_inner_proto_rawDescData)
	})
	return file_inner_proto_rawDescData
}

var file_inner_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_inner_proto_msgTypes = make([]protoimpl.MessageInfo, 15)
var file_inner_proto_goTypes = []interface{}{
	(ApplyCmd_CmdType)(0),  // 0: inner.ApplyCmd.CmdType
	(*JoinReq)(nil),        // 1: inner.JoinReq
	(*JoinRsp)(nil),        // 2: inner.JoinRsp
	(*Member)(nil),         // 3: inner.Member
	(*SynMemberReq)(nil),   // 4: inner.SynMemberReq
	(*SynMemberRsp)(nil),   // 5: inner.SynMemberRsp
	(*TransHttpReq)(nil),   // 6: inner.TransHttpReq
	(*TransHttpRsp)(nil),   // 7: inner.TransHttpRsp
	(*TransGrpcReq)(nil),   // 8: inner.TransGrpcReq
	(*TransGrpcRsp)(nil),   // 9: inner.TransGrpcRsp
	(*HealthReq)(nil),      // 10: inner.HealthReq
	(*HealthRsp)(nil),      // 11: inner.HealthRsp
	(*ApplyCmd)(nil),       // 12: inner.ApplyCmd
	(*ApplyCmd_OpGet)(nil), // 13: inner.ApplyCmd.OpGet
	(*ApplyCmd_OpSet)(nil), // 14: inner.ApplyCmd.OpSet
	(*ApplyCmd_OpDel)(nil), // 15: inner.ApplyCmd.OpDel
}
var file_inner_proto_depIdxs = []int32{
	3,  // 0: inner.SynMemberReq.mem:type_name -> inner.Member
	0,  // 1: inner.ApplyCmd.cmd:type_name -> inner.ApplyCmd.CmdType
	13, // 2: inner.ApplyCmd.get:type_name -> inner.ApplyCmd.OpGet
	14, // 3: inner.ApplyCmd.set:type_name -> inner.ApplyCmd.OpSet
	15, // 4: inner.ApplyCmd.del:type_name -> inner.ApplyCmd.OpDel
	1,  // 5: inner.Raft.JoinRequest:input_type -> inner.JoinReq
	4,  // 6: inner.Raft.SynMember:input_type -> inner.SynMemberReq
	6,  // 7: inner.Raft.TransHttpRequest:input_type -> inner.TransHttpReq
	8,  // 8: inner.Raft.TransGrpcRequest:input_type -> inner.TransGrpcReq
	10, // 9: inner.Raft.HealthRequest:input_type -> inner.HealthReq
	2,  // 10: inner.Raft.JoinRequest:output_type -> inner.JoinRsp
	5,  // 11: inner.Raft.SynMember:output_type -> inner.SynMemberRsp
	7,  // 12: inner.Raft.TransHttpRequest:output_type -> inner.TransHttpRsp
	9,  // 13: inner.Raft.TransGrpcRequest:output_type -> inner.TransGrpcRsp
	11, // 14: inner.Raft.HealthRequest:output_type -> inner.HealthRsp
	10, // [10:15] is the sub-list for method output_type
	5,  // [5:10] is the sub-list for method input_type
	5,  // [5:5] is the sub-list for extension type_name
	5,  // [5:5] is the sub-list for extension extendee
	0,  // [0:5] is the sub-list for field type_name
}

func init() { file_inner_proto_init() }
func file_inner_proto_init() {
	if File_inner_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_inner_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*JoinReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*JoinRsp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Member); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SynMemberReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SynMemberRsp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TransHttpReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TransHttpRsp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TransGrpcReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TransGrpcRsp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HealthReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HealthRsp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[11].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplyCmd); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[12].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplyCmd_OpGet); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[13].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplyCmd_OpSet); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_inner_proto_msgTypes[14].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ApplyCmd_OpDel); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_inner_proto_msgTypes[11].OneofWrappers = []interface{}{
		(*ApplyCmd_Get)(nil),
		(*ApplyCmd_Set)(nil),
		(*ApplyCmd_Del)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_inner_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   15,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_inner_proto_goTypes,
		DependencyIndexes: file_inner_proto_depIdxs,
		EnumInfos:         file_inner_proto_enumTypes,
		MessageInfos:      file_inner_proto_msgTypes,
	}.Build()
	File_inner_proto = out.File
	file_inner_proto_rawDesc = nil
	file_inner_proto_goTypes = nil
	file_inner_proto_depIdxs = nil
}
