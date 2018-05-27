// Code generated by protoc-gen-go. DO NOT EDIT.
// source: netstate.proto

/*
Package netstate is a generated protocol buffer package.

It is generated from these files:
	netstate.proto

It has these top-level messages:
	RedundancyScheme
	EncryptionScheme
	RemotePiece
	RemoteSegment
	Pointer
	PutRequest
	GetRequest
	ListRequest
	PutResponse
	GetResponse
	ListResponse
	DeleteRequest
	DeleteResponse
*/
package netstate

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/timestamp"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type RedundancyScheme_SchemeType int32

const (
	RedundancyScheme_RS RedundancyScheme_SchemeType = 0
)

var RedundancyScheme_SchemeType_name = map[int32]string{
	0: "RS",
}
var RedundancyScheme_SchemeType_value = map[string]int32{
	"RS": 0,
}

func (x RedundancyScheme_SchemeType) String() string {
	return proto.EnumName(RedundancyScheme_SchemeType_name, int32(x))
}
func (RedundancyScheme_SchemeType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor0, []int{0, 0}
}

type EncryptionScheme_EncryptionType int32

const (
	EncryptionScheme_AESGCM    EncryptionScheme_EncryptionType = 0
	EncryptionScheme_SECRETBOX EncryptionScheme_EncryptionType = 1
)

var EncryptionScheme_EncryptionType_name = map[int32]string{
	0: "AESGCM",
	1: "SECRETBOX",
}
var EncryptionScheme_EncryptionType_value = map[string]int32{
	"AESGCM":    0,
	"SECRETBOX": 1,
}

func (x EncryptionScheme_EncryptionType) String() string {
	return proto.EnumName(EncryptionScheme_EncryptionType_name, int32(x))
}
func (EncryptionScheme_EncryptionType) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor0, []int{1, 0}
}

type Pointer_DataType int32

const (
	Pointer_INLINE Pointer_DataType = 0
	Pointer_REMOTE Pointer_DataType = 1
)

var Pointer_DataType_name = map[int32]string{
	0: "INLINE",
	1: "REMOTE",
}
var Pointer_DataType_value = map[string]int32{
	"INLINE": 0,
	"REMOTE": 1,
}

func (x Pointer_DataType) String() string {
	return proto.EnumName(Pointer_DataType_name, int32(x))
}
func (Pointer_DataType) EnumDescriptor() ([]byte, []int) { return fileDescriptor0, []int{4, 0} }

type RedundancyScheme struct {
	Type RedundancyScheme_SchemeType `protobuf:"varint,1,opt,name=type,enum=netstate.RedundancyScheme_SchemeType" json:"type,omitempty"`
	// these values apply to RS encoding
	MinReq           int64 `protobuf:"varint,2,opt,name=min_req,json=minReq" json:"min_req,omitempty"`
	Total            int64 `protobuf:"varint,3,opt,name=total" json:"total,omitempty"`
	RepairThreshold  int64 `protobuf:"varint,4,opt,name=repair_threshold,json=repairThreshold" json:"repair_threshold,omitempty"`
	SuccessThreshold int64 `protobuf:"varint,5,opt,name=success_threshold,json=successThreshold" json:"success_threshold,omitempty"`
}

func (m *RedundancyScheme) Reset()                    { *m = RedundancyScheme{} }
func (m *RedundancyScheme) String() string            { return proto.CompactTextString(m) }
func (*RedundancyScheme) ProtoMessage()               {}
func (*RedundancyScheme) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *RedundancyScheme) GetType() RedundancyScheme_SchemeType {
	if m != nil {
		return m.Type
	}
	return RedundancyScheme_RS
}

func (m *RedundancyScheme) GetMinReq() int64 {
	if m != nil {
		return m.MinReq
	}
	return 0
}

func (m *RedundancyScheme) GetTotal() int64 {
	if m != nil {
		return m.Total
	}
	return 0
}

func (m *RedundancyScheme) GetRepairThreshold() int64 {
	if m != nil {
		return m.RepairThreshold
	}
	return 0
}

func (m *RedundancyScheme) GetSuccessThreshold() int64 {
	if m != nil {
		return m.SuccessThreshold
	}
	return 0
}

type EncryptionScheme struct {
	Type                   EncryptionScheme_EncryptionType `protobuf:"varint,1,opt,name=type,enum=netstate.EncryptionScheme_EncryptionType" json:"type,omitempty"`
	EncryptedEncryptionKey []byte                          `protobuf:"bytes,2,opt,name=encrypted_encryption_key,json=encryptedEncryptionKey,proto3" json:"encrypted_encryption_key,omitempty"`
	EncryptedStartingNonce []byte                          `protobuf:"bytes,3,opt,name=encrypted_starting_nonce,json=encryptedStartingNonce,proto3" json:"encrypted_starting_nonce,omitempty"`
}

func (m *EncryptionScheme) Reset()                    { *m = EncryptionScheme{} }
func (m *EncryptionScheme) String() string            { return proto.CompactTextString(m) }
func (*EncryptionScheme) ProtoMessage()               {}
func (*EncryptionScheme) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *EncryptionScheme) GetType() EncryptionScheme_EncryptionType {
	if m != nil {
		return m.Type
	}
	return EncryptionScheme_AESGCM
}

func (m *EncryptionScheme) GetEncryptedEncryptionKey() []byte {
	if m != nil {
		return m.EncryptedEncryptionKey
	}
	return nil
}

func (m *EncryptionScheme) GetEncryptedStartingNonce() []byte {
	if m != nil {
		return m.EncryptedStartingNonce
	}
	return nil
}

type RemotePiece struct {
	PieceNum int64  `protobuf:"varint,1,opt,name=piece_num,json=pieceNum" json:"piece_num,omitempty"`
	NodeId   string `protobuf:"bytes,2,opt,name=node_id,json=nodeId" json:"node_id,omitempty"`
	Size     int64  `protobuf:"varint,3,opt,name=size" json:"size,omitempty"`
}

func (m *RemotePiece) Reset()                    { *m = RemotePiece{} }
func (m *RemotePiece) String() string            { return proto.CompactTextString(m) }
func (*RemotePiece) ProtoMessage()               {}
func (*RemotePiece) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *RemotePiece) GetPieceNum() int64 {
	if m != nil {
		return m.PieceNum
	}
	return 0
}

func (m *RemotePiece) GetNodeId() string {
	if m != nil {
		return m.NodeId
	}
	return ""
}

func (m *RemotePiece) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

type RemoteSegment struct {
	Redundancy   *RedundancyScheme `protobuf:"bytes,1,opt,name=redundancy" json:"redundancy,omitempty"`
	PieceName    string            `protobuf:"bytes,2,opt,name=piece_name,json=pieceName" json:"piece_name,omitempty"`
	RemotePieces []*RemotePiece    `protobuf:"bytes,3,rep,name=remote_pieces,json=remotePieces" json:"remote_pieces,omitempty"`
	MerkleRoot   []byte            `protobuf:"bytes,4,opt,name=merkle_root,json=merkleRoot,proto3" json:"merkle_root,omitempty"`
	MerkleSize   int64             `protobuf:"varint,5,opt,name=merkle_size,json=merkleSize" json:"merkle_size,omitempty"`
}

func (m *RemoteSegment) Reset()                    { *m = RemoteSegment{} }
func (m *RemoteSegment) String() string            { return proto.CompactTextString(m) }
func (*RemoteSegment) ProtoMessage()               {}
func (*RemoteSegment) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *RemoteSegment) GetRedundancy() *RedundancyScheme {
	if m != nil {
		return m.Redundancy
	}
	return nil
}

func (m *RemoteSegment) GetPieceName() string {
	if m != nil {
		return m.PieceName
	}
	return ""
}

func (m *RemoteSegment) GetRemotePieces() []*RemotePiece {
	if m != nil {
		return m.RemotePieces
	}
	return nil
}

func (m *RemoteSegment) GetMerkleRoot() []byte {
	if m != nil {
		return m.MerkleRoot
	}
	return nil
}

func (m *RemoteSegment) GetMerkleSize() int64 {
	if m != nil {
		return m.MerkleSize
	}
	return 0
}

type Pointer struct {
	Type                     Pointer_DataType           `protobuf:"varint,1,opt,name=type,enum=netstate.Pointer_DataType" json:"type,omitempty"`
	Encryption               *EncryptionScheme          `protobuf:"bytes,2,opt,name=encryption" json:"encryption,omitempty"`
	InlineSegment            []byte                     `protobuf:"bytes,3,opt,name=inline_segment,json=inlineSegment,proto3" json:"inline_segment,omitempty"`
	Remote                   *RemoteSegment             `protobuf:"bytes,4,opt,name=remote" json:"remote,omitempty"`
	EncryptedUnencryptedSize int64                      `protobuf:"varint,5,opt,name=encrypted_unencrypted_size,json=encryptedUnencryptedSize" json:"encrypted_unencrypted_size,omitempty"`
	CreationDate             *google_protobuf.Timestamp `protobuf:"bytes,6,opt,name=creation_date,json=creationDate" json:"creation_date,omitempty"`
	ExpirationDate           *google_protobuf.Timestamp `protobuf:"bytes,7,opt,name=expiration_date,json=expirationDate" json:"expiration_date,omitempty"`
}

func (m *Pointer) Reset()                    { *m = Pointer{} }
func (m *Pointer) String() string            { return proto.CompactTextString(m) }
func (*Pointer) ProtoMessage()               {}
func (*Pointer) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *Pointer) GetType() Pointer_DataType {
	if m != nil {
		return m.Type
	}
	return Pointer_INLINE
}

func (m *Pointer) GetEncryption() *EncryptionScheme {
	if m != nil {
		return m.Encryption
	}
	return nil
}

func (m *Pointer) GetInlineSegment() []byte {
	if m != nil {
		return m.InlineSegment
	}
	return nil
}

func (m *Pointer) GetRemote() *RemoteSegment {
	if m != nil {
		return m.Remote
	}
	return nil
}

func (m *Pointer) GetEncryptedUnencryptedSize() int64 {
	if m != nil {
		return m.EncryptedUnencryptedSize
	}
	return 0
}

func (m *Pointer) GetCreationDate() *google_protobuf.Timestamp {
	if m != nil {
		return m.CreationDate
	}
	return nil
}

func (m *Pointer) GetExpirationDate() *google_protobuf.Timestamp {
	if m != nil {
		return m.ExpirationDate
	}
	return nil
}

// PutRequest is a request message for the Put rpc call
type PutRequest struct {
	Path    []byte   `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
	Pointer *Pointer `protobuf:"bytes,2,opt,name=pointer" json:"pointer,omitempty"`
}

func (m *PutRequest) Reset()                    { *m = PutRequest{} }
func (m *PutRequest) String() string            { return proto.CompactTextString(m) }
func (*PutRequest) ProtoMessage()               {}
func (*PutRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *PutRequest) GetPath() []byte {
	if m != nil {
		return m.Path
	}
	return nil
}

func (m *PutRequest) GetPointer() *Pointer {
	if m != nil {
		return m.Pointer
	}
	return nil
}

// GetRequest is a request message for the Get rpc call
type GetRequest struct {
	Path []byte `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (m *GetRequest) Reset()                    { *m = GetRequest{} }
func (m *GetRequest) String() string            { return proto.CompactTextString(m) }
func (*GetRequest) ProtoMessage()               {}
func (*GetRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *GetRequest) GetPath() []byte {
	if m != nil {
		return m.Path
	}
	return nil
}

// ListRequest is a request message for the List rpc call
type ListRequest struct {
	StartingPathKey []byte `protobuf:"bytes,1,opt,name=starting_path_key,json=startingPathKey,proto3" json:"starting_path_key,omitempty"`
	Limit           int64  `protobuf:"varint,2,opt,name=limit" json:"limit,omitempty"`
}

func (m *ListRequest) Reset()                    { *m = ListRequest{} }
func (m *ListRequest) String() string            { return proto.CompactTextString(m) }
func (*ListRequest) ProtoMessage()               {}
func (*ListRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *ListRequest) GetStartingPathKey() []byte {
	if m != nil {
		return m.StartingPathKey
	}
	return nil
}

func (m *ListRequest) GetLimit() int64 {
	if m != nil {
		return m.Limit
	}
	return 0
}

// PutResponse is a response message for the Put rpc call
type PutResponse struct {
}

func (m *PutResponse) Reset()                    { *m = PutResponse{} }
func (m *PutResponse) String() string            { return proto.CompactTextString(m) }
func (*PutResponse) ProtoMessage()               {}
func (*PutResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{8} }

// GetResponse is a response message for the Get rpc call
type GetResponse struct {
	Pointer []byte `protobuf:"bytes,1,opt,name=pointer,proto3" json:"pointer,omitempty"`
}

func (m *GetResponse) Reset()                    { *m = GetResponse{} }
func (m *GetResponse) String() string            { return proto.CompactTextString(m) }
func (*GetResponse) ProtoMessage()               {}
func (*GetResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{9} }

func (m *GetResponse) GetPointer() []byte {
	if m != nil {
		return m.Pointer
	}
	return nil
}

// ListResponse is a response message for the List rpc call
type ListResponse struct {
	Paths [][]byte `protobuf:"bytes,1,rep,name=paths,proto3" json:"paths,omitempty"`
}

func (m *ListResponse) Reset()                    { *m = ListResponse{} }
func (m *ListResponse) String() string            { return proto.CompactTextString(m) }
func (*ListResponse) ProtoMessage()               {}
func (*ListResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{10} }

func (m *ListResponse) GetPaths() [][]byte {
	if m != nil {
		return m.Paths
	}
	return nil
}

type DeleteRequest struct {
	Path []byte `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (m *DeleteRequest) Reset()                    { *m = DeleteRequest{} }
func (m *DeleteRequest) String() string            { return proto.CompactTextString(m) }
func (*DeleteRequest) ProtoMessage()               {}
func (*DeleteRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{11} }

func (m *DeleteRequest) GetPath() []byte {
	if m != nil {
		return m.Path
	}
	return nil
}

// DeleteResponse is a response message for the Delete rpc call
type DeleteResponse struct {
}

func (m *DeleteResponse) Reset()                    { *m = DeleteResponse{} }
func (m *DeleteResponse) String() string            { return proto.CompactTextString(m) }
func (*DeleteResponse) ProtoMessage()               {}
func (*DeleteResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{12} }

func init() {
	proto.RegisterType((*RedundancyScheme)(nil), "netstate.RedundancyScheme")
	proto.RegisterType((*EncryptionScheme)(nil), "netstate.EncryptionScheme")
	proto.RegisterType((*RemotePiece)(nil), "netstate.RemotePiece")
	proto.RegisterType((*RemoteSegment)(nil), "netstate.RemoteSegment")
	proto.RegisterType((*Pointer)(nil), "netstate.Pointer")
	proto.RegisterType((*PutRequest)(nil), "netstate.PutRequest")
	proto.RegisterType((*GetRequest)(nil), "netstate.GetRequest")
	proto.RegisterType((*ListRequest)(nil), "netstate.ListRequest")
	proto.RegisterType((*PutResponse)(nil), "netstate.PutResponse")
	proto.RegisterType((*GetResponse)(nil), "netstate.GetResponse")
	proto.RegisterType((*ListResponse)(nil), "netstate.ListResponse")
	proto.RegisterType((*DeleteRequest)(nil), "netstate.DeleteRequest")
	proto.RegisterType((*DeleteResponse)(nil), "netstate.DeleteResponse")
	proto.RegisterEnum("netstate.RedundancyScheme_SchemeType", RedundancyScheme_SchemeType_name, RedundancyScheme_SchemeType_value)
	proto.RegisterEnum("netstate.EncryptionScheme_EncryptionType", EncryptionScheme_EncryptionType_name, EncryptionScheme_EncryptionType_value)
	proto.RegisterEnum("netstate.Pointer_DataType", Pointer_DataType_name, Pointer_DataType_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for NetState service

type NetStateClient interface {
	// Put formats and hands off a file path to be saved to boltdb
	Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error)
	// Get formats and hands off a file path to get a small value from boltdb
	Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error)
	// List calls the bolt client's List function and returns all file paths
	List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error)
	// Delete formats and hands off a file path to delete from boltdb
	Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error)
}

type netStateClient struct {
	cc *grpc.ClientConn
}

func NewNetStateClient(cc *grpc.ClientConn) NetStateClient {
	return &netStateClient{cc}
}

func (c *netStateClient) Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error) {
	out := new(PutResponse)
	err := grpc.Invoke(ctx, "/netstate.NetState/Put", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netStateClient) Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error) {
	out := new(GetResponse)
	err := grpc.Invoke(ctx, "/netstate.NetState/Get", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netStateClient) List(ctx context.Context, in *ListRequest, opts ...grpc.CallOption) (*ListResponse, error) {
	out := new(ListResponse)
	err := grpc.Invoke(ctx, "/netstate.NetState/List", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *netStateClient) Delete(ctx context.Context, in *DeleteRequest, opts ...grpc.CallOption) (*DeleteResponse, error) {
	out := new(DeleteResponse)
	err := grpc.Invoke(ctx, "/netstate.NetState/Delete", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for NetState service

type NetStateServer interface {
	// Put formats and hands off a file path to be saved to boltdb
	Put(context.Context, *PutRequest) (*PutResponse, error)
	// Get formats and hands off a file path to get a small value from boltdb
	Get(context.Context, *GetRequest) (*GetResponse, error)
	// List calls the bolt client's List function and returns all file paths
	List(context.Context, *ListRequest) (*ListResponse, error)
	// Delete formats and hands off a file path to delete from boltdb
	Delete(context.Context, *DeleteRequest) (*DeleteResponse, error)
}

func RegisterNetStateServer(s *grpc.Server, srv NetStateServer) {
	s.RegisterService(&_NetState_serviceDesc, srv)
}

func _NetState_Put_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PutRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetStateServer).Put(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/netstate.NetState/Put",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetStateServer).Put(ctx, req.(*PutRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NetState_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetStateServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/netstate.NetState/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetStateServer).Get(ctx, req.(*GetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NetState_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetStateServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/netstate.NetState/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetStateServer).List(ctx, req.(*ListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NetState_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NetStateServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/netstate.NetState/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NetStateServer).Delete(ctx, req.(*DeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _NetState_serviceDesc = grpc.ServiceDesc{
	ServiceName: "netstate.NetState",
	HandlerType: (*NetStateServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Put",
			Handler:    _NetState_Put_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _NetState_Get_Handler,
		},
		{
			MethodName: "List",
			Handler:    _NetState_List_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _NetState_Delete_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "netstate.proto",
}

func init() { proto.RegisterFile("netstate.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 845 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x54, 0x7f, 0x6f, 0xe3, 0x44,
	0x10, 0xad, 0xcf, 0x6d, 0xda, 0x4e, 0x7e, 0xd4, 0x5d, 0xe5, 0xee, 0xac, 0x20, 0x44, 0x64, 0x38,
	0xd1, 0xa3, 0x52, 0x90, 0x82, 0x90, 0xa0, 0x80, 0x10, 0xb4, 0x51, 0x55, 0x71, 0x4d, 0xa3, 0x4d,
	0x10, 0xfc, 0x67, 0xf9, 0xe2, 0x21, 0xb1, 0x2e, 0xde, 0x75, 0xbd, 0x6b, 0x89, 0xf0, 0xbd, 0xf8,
	0x4a, 0x08, 0xf1, 0x27, 0x9f, 0x00, 0x79, 0x77, 0x6d, 0x6f, 0x72, 0xe2, 0xee, 0xaf, 0xec, 0xcc,
	0xbc, 0x99, 0xcc, 0x7b, 0xfb, 0xbc, 0xd0, 0x63, 0x28, 0x85, 0x8c, 0x24, 0x8e, 0xb2, 0x9c, 0x4b,
	0x4e, 0x4e, 0xaa, 0x78, 0x70, 0x26, 0x93, 0x14, 0x85, 0x8c, 0xd2, 0x4c, 0x97, 0x82, 0x7f, 0x1c,
	0xf0, 0x28, 0xc6, 0x05, 0x8b, 0x23, 0xb6, 0xdc, 0xce, 0x97, 0x6b, 0x4c, 0x91, 0x7c, 0x0d, 0x87,
	0x72, 0x9b, 0xa1, 0xef, 0x0c, 0x9d, 0x8b, 0xde, 0xf8, 0xc5, 0xa8, 0x1e, 0xb7, 0x8f, 0x1c, 0xe9,
	0x9f, 0xc5, 0x36, 0x43, 0xaa, 0x5a, 0xc8, 0x73, 0x38, 0x4e, 0x13, 0x16, 0xe6, 0xf8, 0xe8, 0x3f,
	0x19, 0x3a, 0x17, 0x2e, 0x6d, 0xa5, 0x09, 0xa3, 0xf8, 0x48, 0xfa, 0x70, 0x24, 0xb9, 0x8c, 0x36,
	0xbe, 0xab, 0xd2, 0x3a, 0x20, 0x2f, 0xc1, 0xcb, 0x31, 0x8b, 0x92, 0x3c, 0x94, 0xeb, 0x1c, 0xc5,
	0x9a, 0x6f, 0x62, 0xff, 0x50, 0x01, 0xce, 0x74, 0x7e, 0x51, 0xa5, 0xc9, 0x25, 0x9c, 0x8b, 0x62,
	0xb9, 0x44, 0x21, 0x2c, 0xec, 0x91, 0xc2, 0x7a, 0xa6, 0x50, 0x83, 0x83, 0x3e, 0x40, 0xb3, 0x1a,
	0x69, 0xc1, 0x13, 0x3a, 0xf7, 0x0e, 0x82, 0x7f, 0x1d, 0xf0, 0x26, 0x6c, 0x99, 0x6f, 0x33, 0x99,
	0x70, 0x66, 0xc8, 0x7e, 0xb7, 0x43, 0xf6, 0x65, 0x43, 0x76, 0x1f, 0x69, 0x25, 0x2c, 0xc2, 0x5f,
	0x81, 0x8f, 0x3a, 0x8f, 0x71, 0x88, 0x35, 0x22, 0x7c, 0x83, 0x5b, 0xa5, 0x40, 0x87, 0x3e, 0xab,
	0xeb, 0xcd, 0x80, 0x9f, 0x70, 0xbb, 0xdb, 0x29, 0x64, 0x94, 0xcb, 0x84, 0xad, 0x42, 0xc6, 0xd9,
	0x12, 0x95, 0x48, 0x76, 0xe7, 0xdc, 0x94, 0xa7, 0x65, 0x35, 0xb8, 0x84, 0xde, 0xee, 0x2e, 0x04,
	0xa0, 0xf5, 0xc3, 0x64, 0x7e, 0x7b, 0x7d, 0xef, 0x1d, 0x90, 0x2e, 0x9c, 0xce, 0x27, 0xd7, 0x74,
	0xb2, 0xf8, 0xf1, 0xe1, 0x57, 0xcf, 0x09, 0x7e, 0x81, 0x36, 0xc5, 0x94, 0x4b, 0x9c, 0x25, 0xb8,
	0x44, 0xf2, 0x01, 0x9c, 0x66, 0xe5, 0x21, 0x64, 0x45, 0xaa, 0x38, 0xbb, 0xf4, 0x44, 0x25, 0xa6,
	0x45, 0x5a, 0xde, 0x1e, 0xe3, 0x31, 0x86, 0x49, 0xac, 0x76, 0x3f, 0xa5, 0xad, 0x32, 0xbc, 0x8b,
	0x09, 0x81, 0x43, 0x91, 0xfc, 0x81, 0xe6, 0xf2, 0xd4, 0x39, 0xf8, 0xdb, 0x81, 0xae, 0x9e, 0x3c,
	0xc7, 0x55, 0x8a, 0x4c, 0x92, 0x2b, 0x80, 0xbc, 0x76, 0x88, 0x1a, 0xde, 0x1e, 0x0f, 0xfe, 0xdf,
	0x3d, 0xd4, 0x42, 0x93, 0x0f, 0x01, 0xcc, 0x5e, 0x51, 0x8a, 0xe6, 0xdf, 0xf5, 0xa6, 0xd3, 0x28,
	0x45, 0x72, 0x05, 0xdd, 0x5c, 0xfd, 0x57, 0xa8, 0x72, 0xc2, 0x77, 0x87, 0xee, 0x45, 0x7b, 0xfc,
	0xd4, 0x9e, 0x5e, 0x93, 0xa4, 0x9d, 0xbc, 0x09, 0x04, 0xf9, 0x08, 0xda, 0x29, 0xe6, 0x6f, 0x36,
	0x18, 0xe6, 0x9c, 0x4b, 0xe5, 0xaf, 0x0e, 0x05, 0x9d, 0xa2, 0x9c, 0x4b, 0x0b, 0xa0, 0x48, 0x6a,
	0x53, 0x19, 0xc0, 0xbc, 0xa4, 0xfa, 0xa7, 0x0b, 0xc7, 0x33, 0x9e, 0x30, 0x89, 0x39, 0x19, 0xed,
	0xf8, 0xc5, 0xa2, 0x67, 0x00, 0xa3, 0x9b, 0x48, 0x46, 0x96, 0x41, 0xae, 0x00, 0x1a, 0x5b, 0x28,
	0x62, 0x3b, 0xa2, 0xec, 0xbb, 0x8c, 0x5a, 0x68, 0xf2, 0x02, 0x7a, 0x09, 0xdb, 0x24, 0x0c, 0x43,
	0xa1, 0x25, 0x36, 0xc6, 0xe8, 0xea, 0x6c, 0xa5, 0xfb, 0xe7, 0xd0, 0xd2, 0x84, 0x15, 0xb7, 0xf6,
	0xf8, 0xf9, 0xbe, 0x2a, 0x06, 0x48, 0x0d, 0x8c, 0x7c, 0x0b, 0x83, 0xc6, 0x7a, 0x05, 0xb3, 0x6c,
	0xd8, 0xf0, 0x6f, 0xcc, 0xf9, 0x73, 0x03, 0x28, 0xd5, 0x20, 0xdf, 0x43, 0x77, 0x99, 0x63, 0xa4,
	0x6c, 0x1e, 0x47, 0x12, 0xfd, 0x96, 0x21, 0xb5, 0xe2, 0x7c, 0xb5, 0x31, 0x8f, 0xce, 0xeb, 0xe2,
	0xb7, 0xd1, 0xa2, 0x7a, 0x6c, 0x68, 0xa7, 0x6a, 0xb8, 0x89, 0x24, 0x92, 0x6b, 0x38, 0xc3, 0xdf,
	0xb3, 0x24, 0xb7, 0x46, 0x1c, 0xbf, 0x77, 0x44, 0xaf, 0x69, 0x29, 0x87, 0x04, 0x01, 0x9c, 0x54,
	0x4a, 0x97, 0xf6, 0xbf, 0x9b, 0xbe, 0xba, 0x9b, 0x4e, 0xbc, 0x83, 0xf2, 0x4c, 0x27, 0xf7, 0x0f,
	0x8b, 0x89, 0xe7, 0x04, 0xf7, 0x00, 0xb3, 0x42, 0x52, 0x7c, 0x2c, 0x50, 0xc8, 0xd2, 0xc4, 0x59,
	0x24, 0xd7, 0xea, 0xe6, 0x3a, 0x54, 0x9d, 0xc9, 0x25, 0x1c, 0x67, 0xfa, 0xde, 0xcc, 0xd5, 0x9c,
	0xbf, 0x75, 0xa1, 0xb4, 0x42, 0x04, 0x43, 0x80, 0x5b, 0x7c, 0xd7, 0xb8, 0xe0, 0x01, 0xda, 0xaf,
	0x12, 0x51, 0x43, 0x3e, 0x83, 0xf3, 0xfa, 0xc3, 0x2e, 0xeb, 0xea, 0x55, 0xd0, 0xf8, 0xb3, 0xaa,
	0x30, 0x8b, 0xe4, 0xba, 0x7c, 0x0e, 0xfa, 0x70, 0xb4, 0x49, 0xd2, 0x44, 0x9a, 0x77, 0x53, 0x07,
	0x41, 0x17, 0xda, 0x8a, 0x81, 0xc8, 0x38, 0x13, 0x18, 0x7c, 0x0a, 0x6d, 0xb5, 0x81, 0x0e, 0x89,
	0xdf, 0x6c, 0xaf, 0xa7, 0xd6, 0xab, 0x7e, 0x02, 0x1d, 0xbd, 0x88, 0x41, 0xf6, 0xe1, 0xa8, 0x5c,
	0x40, 0xf8, 0xce, 0xd0, 0xbd, 0xe8, 0x50, 0x1d, 0x04, 0x1f, 0x43, 0xf7, 0x06, 0x37, 0x28, 0xf1,
	0x5d, 0x9c, 0x3c, 0xe8, 0x55, 0x20, 0x3d, 0x6c, 0xfc, 0x97, 0x03, 0x27, 0x53, 0x94, 0xf3, 0x52,
	0x25, 0x32, 0x06, 0x77, 0x56, 0x48, 0xd2, 0xb7, 0x74, 0xab, 0x25, 0x1f, 0x3c, 0xdd, 0xcb, 0x9a,
	0x6d, 0xc6, 0xe0, 0xde, 0xe2, 0x4e, 0x4f, 0xa3, 0xab, 0xdd, 0x63, 0x73, 0xfd, 0x12, 0x0e, 0x4b,
	0x46, 0xc4, 0x2a, 0x5b, 0x52, 0x0f, 0x9e, 0xed, 0xa7, 0x4d, 0xdb, 0x37, 0xd0, 0xd2, 0xdb, 0x13,
	0xeb, 0xab, 0xd8, 0x21, 0x3d, 0xf0, 0xdf, 0x2e, 0xe8, 0xe6, 0xd7, 0x2d, 0xe5, 0xc3, 0x2f, 0xfe,
	0x0b, 0x00, 0x00, 0xff, 0xff, 0xa6, 0xd4, 0x49, 0x46, 0x51, 0x07, 0x00, 0x00,
}
