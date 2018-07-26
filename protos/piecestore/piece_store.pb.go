// Code generated by protoc-gen-go.
// source: piece_store.proto
// DO NOT EDIT!

/*
Package piecestoreroutes is a generated protocol buffer package.

It is generated from these files:
	piece_store.proto

It has these top-level messages:
	PieceStore
	PieceId
	PieceSummary
	PieceRetrieval
	PieceRetrievalStream
	PieceDelete
	PieceDeleteSummary
	PieceStoreSummary
*/
package piecestoreroutes

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

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

type PieceStore struct {
	Id      string `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Ttl     int64  `protobuf:"varint,2,opt,name=ttl" json:"ttl,omitempty"`
	Content []byte `protobuf:"bytes,3,opt,name=content,proto3" json:"content,omitempty"`
}

func (m *PieceStore) Reset()                    { *m = PieceStore{} }
func (m *PieceStore) String() string            { return proto.CompactTextString(m) }
func (*PieceStore) ProtoMessage()               {}
func (*PieceStore) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *PieceStore) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PieceStore) GetTtl() int64 {
	if m != nil {
		return m.Ttl
	}
	return 0
}

func (m *PieceStore) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

type PieceId struct {
	Id string `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
}

func (m *PieceId) Reset()                    { *m = PieceId{} }
func (m *PieceId) String() string            { return proto.CompactTextString(m) }
func (*PieceId) ProtoMessage()               {}
func (*PieceId) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *PieceId) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

type PieceSummary struct {
	Id         string `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Size       int64  `protobuf:"varint,2,opt,name=size" json:"size,omitempty"`
	Expiration int64  `protobuf:"varint,3,opt,name=expiration" json:"expiration,omitempty"`
}

func (m *PieceSummary) Reset()                    { *m = PieceSummary{} }
func (m *PieceSummary) String() string            { return proto.CompactTextString(m) }
func (*PieceSummary) ProtoMessage()               {}
func (*PieceSummary) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *PieceSummary) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PieceSummary) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *PieceSummary) GetExpiration() int64 {
	if m != nil {
		return m.Expiration
	}
	return 0
}

type PieceRetrieval struct {
	Id     string `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
	Size   int64  `protobuf:"varint,2,opt,name=size" json:"size,omitempty"`
	Offset int64  `protobuf:"varint,3,opt,name=offset" json:"offset,omitempty"`
}

func (m *PieceRetrieval) Reset()                    { *m = PieceRetrieval{} }
func (m *PieceRetrieval) String() string            { return proto.CompactTextString(m) }
func (*PieceRetrieval) ProtoMessage()               {}
func (*PieceRetrieval) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *PieceRetrieval) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

func (m *PieceRetrieval) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *PieceRetrieval) GetOffset() int64 {
	if m != nil {
		return m.Offset
	}
	return 0
}

type PieceRetrievalStream struct {
	Size    int64  `protobuf:"varint,1,opt,name=size" json:"size,omitempty"`
	Content []byte `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
}

func (m *PieceRetrievalStream) Reset()                    { *m = PieceRetrievalStream{} }
func (m *PieceRetrievalStream) String() string            { return proto.CompactTextString(m) }
func (*PieceRetrievalStream) ProtoMessage()               {}
func (*PieceRetrievalStream) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *PieceRetrievalStream) GetSize() int64 {
	if m != nil {
		return m.Size
	}
	return 0
}

func (m *PieceRetrievalStream) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

type PieceDelete struct {
	Id string `protobuf:"bytes,1,opt,name=id" json:"id,omitempty"`
}

func (m *PieceDelete) Reset()                    { *m = PieceDelete{} }
func (m *PieceDelete) String() string            { return proto.CompactTextString(m) }
func (*PieceDelete) ProtoMessage()               {}
func (*PieceDelete) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *PieceDelete) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

type PieceDeleteSummary struct {
	Message string `protobuf:"bytes,1,opt,name=message" json:"message,omitempty"`
}

func (m *PieceDeleteSummary) Reset()                    { *m = PieceDeleteSummary{} }
func (m *PieceDeleteSummary) String() string            { return proto.CompactTextString(m) }
func (*PieceDeleteSummary) ProtoMessage()               {}
func (*PieceDeleteSummary) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

func (m *PieceDeleteSummary) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

type PieceStoreSummary struct {
	Message       string `protobuf:"bytes,1,opt,name=message" json:"message,omitempty"`
	TotalReceived int64  `protobuf:"varint,2,opt,name=totalReceived" json:"totalReceived,omitempty"`
}

func (m *PieceStoreSummary) Reset()                    { *m = PieceStoreSummary{} }
func (m *PieceStoreSummary) String() string            { return proto.CompactTextString(m) }
func (*PieceStoreSummary) ProtoMessage()               {}
func (*PieceStoreSummary) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{7} }

func (m *PieceStoreSummary) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func (m *PieceStoreSummary) GetTotalReceived() int64 {
	if m != nil {
		return m.TotalReceived
	}
	return 0
}

func init() {
	proto.RegisterType((*PieceStore)(nil), "piecestoreroutes.PieceStore")
	proto.RegisterType((*PieceId)(nil), "piecestoreroutes.PieceId")
	proto.RegisterType((*PieceSummary)(nil), "piecestoreroutes.PieceSummary")
	proto.RegisterType((*PieceRetrieval)(nil), "piecestoreroutes.PieceRetrieval")
	proto.RegisterType((*PieceRetrievalStream)(nil), "piecestoreroutes.PieceRetrievalStream")
	proto.RegisterType((*PieceDelete)(nil), "piecestoreroutes.PieceDelete")
	proto.RegisterType((*PieceDeleteSummary)(nil), "piecestoreroutes.PieceDeleteSummary")
	proto.RegisterType((*PieceStoreSummary)(nil), "piecestoreroutes.PieceStoreSummary")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for PieceStoreRoutes service

type PieceStoreRoutesClient interface {
	Piece(ctx context.Context, in *PieceId, opts ...grpc.CallOption) (*PieceSummary, error)
	Retrieve(ctx context.Context, in *PieceRetrieval, opts ...grpc.CallOption) (PieceStoreRoutes_RetrieveClient, error)
	Store(ctx context.Context, opts ...grpc.CallOption) (PieceStoreRoutes_StoreClient, error)
	Delete(ctx context.Context, in *PieceDelete, opts ...grpc.CallOption) (*PieceDeleteSummary, error)
}

type pieceStoreRoutesClient struct {
	cc *grpc.ClientConn
}

func NewPieceStoreRoutesClient(cc *grpc.ClientConn) PieceStoreRoutesClient {
	return &pieceStoreRoutesClient{cc}
}

func (c *pieceStoreRoutesClient) Piece(ctx context.Context, in *PieceId, opts ...grpc.CallOption) (*PieceSummary, error) {
	out := new(PieceSummary)
	err := grpc.Invoke(ctx, "/piecestoreroutes.PieceStoreRoutes/Piece", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *pieceStoreRoutesClient) Retrieve(ctx context.Context, in *PieceRetrieval, opts ...grpc.CallOption) (PieceStoreRoutes_RetrieveClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_PieceStoreRoutes_serviceDesc.Streams[0], c.cc, "/piecestoreroutes.PieceStoreRoutes/Retrieve", opts...)
	if err != nil {
		return nil, err
	}
	x := &pieceStoreRoutesRetrieveClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type PieceStoreRoutes_RetrieveClient interface {
	Recv() (*PieceRetrievalStream, error)
	grpc.ClientStream
}

type pieceStoreRoutesRetrieveClient struct {
	grpc.ClientStream
}

func (x *pieceStoreRoutesRetrieveClient) Recv() (*PieceRetrievalStream, error) {
	m := new(PieceRetrievalStream)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *pieceStoreRoutesClient) Store(ctx context.Context, opts ...grpc.CallOption) (PieceStoreRoutes_StoreClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_PieceStoreRoutes_serviceDesc.Streams[1], c.cc, "/piecestoreroutes.PieceStoreRoutes/Store", opts...)
	if err != nil {
		return nil, err
	}
	x := &pieceStoreRoutesStoreClient{stream}
	return x, nil
}

type PieceStoreRoutes_StoreClient interface {
	Send(*PieceStore) error
	CloseAndRecv() (*PieceStoreSummary, error)
	grpc.ClientStream
}

type pieceStoreRoutesStoreClient struct {
	grpc.ClientStream
}

func (x *pieceStoreRoutesStoreClient) Send(m *PieceStore) error {
	return x.ClientStream.SendMsg(m)
}

func (x *pieceStoreRoutesStoreClient) CloseAndRecv() (*PieceStoreSummary, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(PieceStoreSummary)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *pieceStoreRoutesClient) Delete(ctx context.Context, in *PieceDelete, opts ...grpc.CallOption) (*PieceDeleteSummary, error) {
	out := new(PieceDeleteSummary)
	err := grpc.Invoke(ctx, "/piecestoreroutes.PieceStoreRoutes/Delete", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for PieceStoreRoutes service

type PieceStoreRoutesServer interface {
	Piece(context.Context, *PieceId) (*PieceSummary, error)
	Retrieve(*PieceRetrieval, PieceStoreRoutes_RetrieveServer) error
	Store(PieceStoreRoutes_StoreServer) error
	Delete(context.Context, *PieceDelete) (*PieceDeleteSummary, error)
}

func RegisterPieceStoreRoutesServer(s *grpc.Server, srv PieceStoreRoutesServer) {
	s.RegisterService(&_PieceStoreRoutes_serviceDesc, srv)
}

func _PieceStoreRoutes_Piece_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PieceId)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PieceStoreRoutesServer).Piece(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/piecestoreroutes.PieceStoreRoutes/Piece",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PieceStoreRoutesServer).Piece(ctx, req.(*PieceId))
	}
	return interceptor(ctx, in, info, handler)
}

func _PieceStoreRoutes_Retrieve_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(PieceRetrieval)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(PieceStoreRoutesServer).Retrieve(m, &pieceStoreRoutesRetrieveServer{stream})
}

type PieceStoreRoutes_RetrieveServer interface {
	Send(*PieceRetrievalStream) error
	grpc.ServerStream
}

type pieceStoreRoutesRetrieveServer struct {
	grpc.ServerStream
}

func (x *pieceStoreRoutesRetrieveServer) Send(m *PieceRetrievalStream) error {
	return x.ServerStream.SendMsg(m)
}

func _PieceStoreRoutes_Store_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(PieceStoreRoutesServer).Store(&pieceStoreRoutesStoreServer{stream})
}

type PieceStoreRoutes_StoreServer interface {
	SendAndClose(*PieceStoreSummary) error
	Recv() (*PieceStore, error)
	grpc.ServerStream
}

type pieceStoreRoutesStoreServer struct {
	grpc.ServerStream
}

func (x *pieceStoreRoutesStoreServer) SendAndClose(m *PieceStoreSummary) error {
	return x.ServerStream.SendMsg(m)
}

func (x *pieceStoreRoutesStoreServer) Recv() (*PieceStore, error) {
	m := new(PieceStore)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func _PieceStoreRoutes_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PieceDelete)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PieceStoreRoutesServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/piecestoreroutes.PieceStoreRoutes/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PieceStoreRoutesServer).Delete(ctx, req.(*PieceDelete))
	}
	return interceptor(ctx, in, info, handler)
}

var _PieceStoreRoutes_serviceDesc = grpc.ServiceDesc{
	ServiceName: "piecestoreroutes.PieceStoreRoutes",
	HandlerType: (*PieceStoreRoutesServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Piece",
			Handler:    _PieceStoreRoutes_Piece_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _PieceStoreRoutes_Delete_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Retrieve",
			Handler:       _PieceStoreRoutes_Retrieve_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Store",
			Handler:       _PieceStoreRoutes_Store_Handler,
			ClientStreams: true,
		},
	},
	Metadata: "piece_store.proto",
}

func init() { proto.RegisterFile("piece_store.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 369 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x8c, 0x93, 0x51, 0x4f, 0xe2, 0x40,
	0x10, 0xc7, 0x69, 0x7b, 0xc0, 0xdd, 0x1c, 0x47, 0x60, 0x73, 0x31, 0xa5, 0x11, 0xd2, 0xac, 0xc4,
	0xf4, 0xa9, 0x31, 0xfa, 0x15, 0x78, 0xd0, 0xc4, 0xa8, 0x59, 0x5e, 0x7c, 0x33, 0x95, 0x0e, 0x66,
	0x93, 0x96, 0x25, 0xdb, 0x85, 0xa8, 0x5f, 0xd3, 0x2f, 0x64, 0xba, 0x5d, 0x4a, 0x81, 0x14, 0x7c,
	0xdb, 0xf9, 0xcf, 0xf0, 0x9b, 0xe1, 0x3f, 0x53, 0xe8, 0x2f, 0x39, 0xce, 0xf0, 0x25, 0x53, 0x42,
	0x62, 0xb8, 0x94, 0x42, 0x09, 0xd2, 0xd3, 0x92, 0x56, 0xa4, 0x58, 0x29, 0xcc, 0xe8, 0x2d, 0xc0,
	0x53, 0xae, 0x4d, 0x73, 0x8d, 0x74, 0xc1, 0xe6, 0xb1, 0x6b, 0xf9, 0x56, 0xf0, 0x87, 0xd9, 0x3c,
	0x26, 0x3d, 0x70, 0x94, 0x4a, 0x5c, 0xdb, 0xb7, 0x02, 0x87, 0xe5, 0x4f, 0xe2, 0x42, 0x7b, 0x26,
	0x16, 0x0a, 0x17, 0xca, 0x75, 0x7c, 0x2b, 0xe8, 0xb0, 0x4d, 0x48, 0x07, 0xd0, 0xd6, 0xa4, 0xbb,
	0x78, 0x1f, 0x43, 0x19, 0x74, 0x8a, 0x26, 0xab, 0x34, 0x8d, 0xe4, 0xc7, 0x41, 0x1b, 0x02, 0xbf,
	0x32, 0xfe, 0x89, 0xa6, 0x8f, 0x7e, 0x93, 0x11, 0x00, 0xbe, 0x2f, 0xb9, 0x8c, 0x14, 0x17, 0x0b,
	0xdd, 0xcb, 0x61, 0x15, 0x85, 0xde, 0x43, 0x57, 0x33, 0x19, 0x2a, 0xc9, 0x71, 0x1d, 0x25, 0x3f,
	0xa2, 0x9e, 0x41, 0x4b, 0xcc, 0xe7, 0x19, 0x2a, 0x43, 0x34, 0x11, 0x9d, 0xc0, 0xff, 0x5d, 0xda,
	0x54, 0x49, 0x8c, 0xd2, 0x92, 0x61, 0x55, 0x18, 0x15, 0x0b, 0xec, 0x5d, 0x0b, 0x86, 0xf0, 0x57,
	0x53, 0x26, 0x98, 0xa0, 0x3a, 0x70, 0x93, 0x86, 0x40, 0x2a, 0xe9, 0x8d, 0x19, 0x2e, 0xb4, 0x53,
	0xcc, 0xb2, 0xe8, 0x0d, 0x4d, 0xe9, 0x26, 0xa4, 0x53, 0xe8, 0x6f, 0x77, 0x73, 0xb2, 0x9c, 0x8c,
	0xe1, 0x9f, 0x12, 0x2a, 0x4a, 0x18, 0xce, 0x90, 0xaf, 0x31, 0x36, 0x7f, 0x7c, 0x57, 0xbc, 0xfe,
	0xb2, 0xa1, 0xb7, 0xa5, 0x32, 0x7d, 0x05, 0x64, 0x02, 0x4d, 0xad, 0x91, 0x41, 0xb8, 0x7f, 0x21,
	0xa1, 0x59, 0xaa, 0x37, 0xaa, 0x49, 0x99, 0xc1, 0x68, 0x83, 0x3c, 0xc3, 0x6f, 0xe3, 0x1f, 0x12,
	0xbf, 0xa6, 0xba, 0x34, 0xd8, 0xbb, 0x3c, 0x55, 0x51, 0xac, 0x80, 0x36, 0xae, 0x2c, 0xf2, 0x00,
	0xcd, 0xe2, 0x40, 0xcf, 0xeb, 0x86, 0xc8, 0x05, 0xef, 0xe2, 0x58, 0xb6, 0x9c, 0x33, 0xb0, 0xc8,
	0x23, 0xb4, 0xcc, 0x8e, 0x86, 0x35, 0x3f, 0x29, 0xd2, 0xde, 0xf8, 0x68, 0xba, 0x44, 0xbe, 0xb6,
	0xf4, 0xf7, 0x75, 0xf3, 0x1d, 0x00, 0x00, 0xff, 0xff, 0xf0, 0xca, 0x86, 0xb6, 0x74, 0x03, 0x00,
	0x00,
}
