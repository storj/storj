// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: contact.proto

package pb

import (
	context "context"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	math "math"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type CheckinRequest struct {
	Sender               *Node         `protobuf:"bytes,1,opt,name=sender,proto3" json:"sender,omitempty"`
	Capacity             *NodeCapacity `protobuf:"bytes,2,opt,name=capacity,proto3" json:"capacity,omitempty"`
	Operator             *NodeOperator `protobuf:"bytes,3,opt,name=operator,proto3" json:"operator,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *CheckinRequest) Reset()         { *m = CheckinRequest{} }
func (m *CheckinRequest) String() string { return proto.CompactTextString(m) }
func (*CheckinRequest) ProtoMessage()    {}
func (*CheckinRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5036fff2565fb15, []int{0}
}
func (m *CheckinRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckinRequest.Unmarshal(m, b)
}
func (m *CheckinRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckinRequest.Marshal(b, m, deterministic)
}
func (m *CheckinRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckinRequest.Merge(m, src)
}
func (m *CheckinRequest) XXX_Size() int {
	return xxx_messageInfo_CheckinRequest.Size(m)
}
func (m *CheckinRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckinRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CheckinRequest proto.InternalMessageInfo

func (m *CheckinRequest) GetSender() *Node {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *CheckinRequest) GetCapacity() *NodeCapacity {
	if m != nil {
		return m.Capacity
	}
	return nil
}

func (m *CheckinRequest) GetOperator() *NodeOperator {
	if m != nil {
		return m.Operator
	}
	return nil
}

type CheckinResponse struct {
	PingNodeSuccess      bool     `protobuf:"varint,1,opt,name=ping_node_success,json=pingNodeSuccess,proto3" json:"ping_node_success,omitempty"`
	ErrorMessage         string   `protobuf:"bytes,2,opt,name=error_message,json=errorMessage,proto3" json:"error_message,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CheckinResponse) Reset()         { *m = CheckinResponse{} }
func (m *CheckinResponse) String() string { return proto.CompactTextString(m) }
func (*CheckinResponse) ProtoMessage()    {}
func (*CheckinResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5036fff2565fb15, []int{1}
}
func (m *CheckinResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckinResponse.Unmarshal(m, b)
}
func (m *CheckinResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckinResponse.Marshal(b, m, deterministic)
}
func (m *CheckinResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckinResponse.Merge(m, src)
}
func (m *CheckinResponse) XXX_Size() int {
	return xxx_messageInfo_CheckinResponse.Size(m)
}
func (m *CheckinResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckinResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CheckinResponse proto.InternalMessageInfo

func (m *CheckinResponse) GetPingNodeSuccess() bool {
	if m != nil {
		return m.PingNodeSuccess
	}
	return false
}

func (m *CheckinResponse) GetErrorMessage() string {
	if m != nil {
		return m.ErrorMessage
	}
	return ""
}

type PingNodeRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PingNodeRequest) Reset()         { *m = PingNodeRequest{} }
func (m *PingNodeRequest) String() string { return proto.CompactTextString(m) }
func (*PingNodeRequest) ProtoMessage()    {}
func (*PingNodeRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5036fff2565fb15, []int{2}
}
func (m *PingNodeRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PingNodeRequest.Unmarshal(m, b)
}
func (m *PingNodeRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PingNodeRequest.Marshal(b, m, deterministic)
}
func (m *PingNodeRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PingNodeRequest.Merge(m, src)
}
func (m *PingNodeRequest) XXX_Size() int {
	return xxx_messageInfo_PingNodeRequest.Size(m)
}
func (m *PingNodeRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_PingNodeRequest.DiscardUnknown(m)
}

var xxx_messageInfo_PingNodeRequest proto.InternalMessageInfo

type PingNodeResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *PingNodeResponse) Reset()         { *m = PingNodeResponse{} }
func (m *PingNodeResponse) String() string { return proto.CompactTextString(m) }
func (*PingNodeResponse) ProtoMessage()    {}
func (*PingNodeResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5036fff2565fb15, []int{3}
}
func (m *PingNodeResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PingNodeResponse.Unmarshal(m, b)
}
func (m *PingNodeResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PingNodeResponse.Marshal(b, m, deterministic)
}
func (m *PingNodeResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PingNodeResponse.Merge(m, src)
}
func (m *PingNodeResponse) XXX_Size() int {
	return xxx_messageInfo_PingNodeResponse.Size(m)
}
func (m *PingNodeResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_PingNodeResponse.DiscardUnknown(m)
}

var xxx_messageInfo_PingNodeResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*CheckinRequest)(nil), "contact.CheckinRequest")
	proto.RegisterType((*CheckinResponse)(nil), "contact.CheckinResponse")
	proto.RegisterType((*PingNodeRequest)(nil), "contact.PingNodeRequest")
	proto.RegisterType((*PingNodeResponse)(nil), "contact.PingNodeResponse")
}

func init() { proto.RegisterFile("contact.proto", fileDescriptor_a5036fff2565fb15) }

var fileDescriptor_a5036fff2565fb15 = []byte{
	// 285 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x91, 0x3f, 0x4f, 0xc3, 0x30,
	0x10, 0xc5, 0xd5, 0x52, 0x35, 0xe1, 0xa0, 0x84, 0x7a, 0x21, 0x64, 0x42, 0x61, 0x41, 0x0c, 0x19,
	0xca, 0x8a, 0x84, 0x44, 0x98, 0x90, 0xf8, 0x23, 0xb3, 0xb1, 0x44, 0x89, 0x73, 0x0a, 0x11, 0xc2,
	0x67, 0x6c, 0x77, 0xe0, 0x7b, 0xf0, 0x81, 0x51, 0x6c, 0x37, 0x15, 0x85, 0x2d, 0x79, 0xef, 0x77,
	0xcf, 0xf7, 0x07, 0x16, 0x82, 0xa4, 0xad, 0x85, 0x2d, 0x94, 0x26, 0x4b, 0x2c, 0x0a, 0xbf, 0x19,
	0x74, 0xd4, 0x91, 0x17, 0x33, 0x90, 0xd4, 0xa2, 0xff, 0xce, 0xbf, 0x27, 0x70, 0x54, 0xbe, 0xa1,
	0x78, 0xef, 0x25, 0xc7, 0xcf, 0x35, 0x1a, 0xcb, 0x72, 0x98, 0x1b, 0x94, 0x2d, 0xea, 0x74, 0x72,
	0x36, 0xb9, 0x38, 0x58, 0x41, 0xe1, 0xf8, 0x47, 0x6a, 0x91, 0x07, 0x87, 0x15, 0x10, 0x8b, 0x5a,
	0xd5, 0xa2, 0xb7, 0x5f, 0xe9, 0xd4, 0x51, 0x6c, 0x4b, 0x95, 0xc1, 0xe1, 0x23, 0x33, 0xf0, 0xa4,
	0x50, 0xd7, 0x96, 0x74, 0xba, 0xb7, 0xcb, 0x3f, 0x05, 0x87, 0x8f, 0x4c, 0xde, 0x40, 0x32, 0x76,
	0x65, 0x14, 0x49, 0x83, 0xec, 0x12, 0x96, 0xaa, 0x97, 0x5d, 0x35, 0x94, 0x55, 0x66, 0x2d, 0x04,
	0x1a, 0xe3, 0x3a, 0x8c, 0x79, 0x32, 0x18, 0x43, 0xd2, 0x8b, 0x97, 0xd9, 0x39, 0x2c, 0x50, 0x6b,
	0xd2, 0xd5, 0x07, 0x1a, 0x53, 0x77, 0xe8, 0x7a, 0xdc, 0xe7, 0x87, 0x4e, 0x7c, 0xf0, 0x5a, 0xbe,
	0x84, 0xe4, 0x39, 0xd4, 0x85, 0xd1, 0x73, 0x06, 0xc7, 0x5b, 0xc9, 0xbf, 0xbb, 0xba, 0x87, 0xa8,
	0xf4, 0x4b, 0x64, 0x37, 0x10, 0x6f, 0x6c, 0x96, 0x16, 0x9b, 0x4d, 0xef, 0x84, 0x64, 0xa7, 0xff,
	0x38, 0x21, 0xeb, 0x0e, 0x66, 0xae, 0xf8, 0x1a, 0xa2, 0x30, 0x1e, 0x3b, 0x19, 0xe9, 0xdf, 0x67,
	0xc8, 0xd2, 0xbf, 0x86, 0x4f, 0xb9, 0x9d, 0xbd, 0x4e, 0x55, 0xd3, 0xcc, 0xdd, 0x01, 0xaf, 0x7e,
	0x02, 0x00, 0x00, 0xff, 0xff, 0x77, 0xb2, 0xa2, 0xcb, 0xf2, 0x01, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ContactClient is the client API for Contact service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ContactClient interface {
	PingNode(ctx context.Context, in *PingNodeRequest, opts ...grpc.CallOption) (*PingNodeResponse, error)
}

type contactClient struct {
	cc *grpc.ClientConn
}

func NewContactClient(cc *grpc.ClientConn) ContactClient {
	return &contactClient{cc}
}

func (c *contactClient) PingNode(ctx context.Context, in *PingNodeRequest, opts ...grpc.CallOption) (*PingNodeResponse, error) {
	out := new(PingNodeResponse)
	err := c.cc.Invoke(ctx, "/contact.Contact/PingNode", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ContactServer is the server API for Contact service.
type ContactServer interface {
	PingNode(context.Context, *PingNodeRequest) (*PingNodeResponse, error)
}

// UnimplementedContactServer can be embedded to have forward compatible implementations.
type UnimplementedContactServer struct {
}

func (*UnimplementedContactServer) PingNode(ctx context.Context, req *PingNodeRequest) (*PingNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method PingNode not implemented")
}

func RegisterContactServer(s *grpc.Server, srv ContactServer) {
	s.RegisterService(&_Contact_serviceDesc, srv)
}

func _Contact_PingNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingNodeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ContactServer).PingNode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/contact.Contact/PingNode",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ContactServer).PingNode(ctx, req.(*PingNodeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Contact_serviceDesc = grpc.ServiceDesc{
	ServiceName: "contact.Contact",
	HandlerType: (*ContactServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "PingNode",
			Handler:    _Contact_PingNode_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "contact.proto",
}

// NodeClient is the client API for Node service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type NodeClient interface {
	Checkin(ctx context.Context, in *CheckinRequest, opts ...grpc.CallOption) (*CheckinResponse, error)
}

type nodeClient struct {
	cc *grpc.ClientConn
}

func NewNodeClient(cc *grpc.ClientConn) NodeClient {
	return &nodeClient{cc}
}

func (c *nodeClient) Checkin(ctx context.Context, in *CheckinRequest, opts ...grpc.CallOption) (*CheckinResponse, error) {
	out := new(CheckinResponse)
	err := c.cc.Invoke(ctx, "/contact.Node/Checkin", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NodeServer is the server API for Node service.
type NodeServer interface {
	Checkin(context.Context, *CheckinRequest) (*CheckinResponse, error)
}

// UnimplementedNodeServer can be embedded to have forward compatible implementations.
type UnimplementedNodeServer struct {
}

func (*UnimplementedNodeServer) Checkin(ctx context.Context, req *CheckinRequest) (*CheckinResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Checkin not implemented")
}

func RegisterNodeServer(s *grpc.Server, srv NodeServer) {
	s.RegisterService(&_Node_serviceDesc, srv)
}

func _Node_Checkin_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckinRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeServer).Checkin(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/contact.Node/Checkin",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeServer).Checkin(ctx, req.(*CheckinRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Node_serviceDesc = grpc.ServiceDesc{
	ServiceName: "contact.Node",
	HandlerType: (*NodeServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Checkin",
			Handler:    _Node_Checkin_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "contact.proto",
}
