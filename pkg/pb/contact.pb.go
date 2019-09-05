// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: contact.proto

package pb

import (
	context "context"
	fmt "fmt"
	proto "github.com/gogo/protobuf/proto"
	grpc "google.golang.org/grpc"
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
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type CheckinRequest struct {
	Address              *NodeAddress  `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Capacity             *NodeCapacity `protobuf:"bytes,3,opt,name=capacity,proto3" json:"capacity,omitempty"`
	Operator             *NodeOperator `protobuf:"bytes,4,opt,name=operator,proto3" json:"operator,omitempty"`
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

func (m *CheckinRequest) GetAddress() *NodeAddress {
	if m != nil {
		return m.Address
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
	PingErrorMessage     string   `protobuf:"bytes,2,opt,name=ping_error_message,json=pingErrorMessage,proto3" json:"ping_error_message,omitempty"`
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

func (m *CheckinResponse) GetPingErrorMessage() string {
	if m != nil {
		return m.PingErrorMessage
	}
	return ""
}

type ContactPingRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ContactPingRequest) Reset()         { *m = ContactPingRequest{} }
func (m *ContactPingRequest) String() string { return proto.CompactTextString(m) }
func (*ContactPingRequest) ProtoMessage()    {}
func (*ContactPingRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5036fff2565fb15, []int{2}
}
func (m *ContactPingRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ContactPingRequest.Unmarshal(m, b)
}
func (m *ContactPingRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ContactPingRequest.Marshal(b, m, deterministic)
}
func (m *ContactPingRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ContactPingRequest.Merge(m, src)
}
func (m *ContactPingRequest) XXX_Size() int {
	return xxx_messageInfo_ContactPingRequest.Size(m)
}
func (m *ContactPingRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ContactPingRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ContactPingRequest proto.InternalMessageInfo

type ContactPingResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ContactPingResponse) Reset()         { *m = ContactPingResponse{} }
func (m *ContactPingResponse) String() string { return proto.CompactTextString(m) }
func (*ContactPingResponse) ProtoMessage()    {}
func (*ContactPingResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_a5036fff2565fb15, []int{3}
}
func (m *ContactPingResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ContactPingResponse.Unmarshal(m, b)
}
func (m *ContactPingResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ContactPingResponse.Marshal(b, m, deterministic)
}
func (m *ContactPingResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ContactPingResponse.Merge(m, src)
}
func (m *ContactPingResponse) XXX_Size() int {
	return xxx_messageInfo_ContactPingResponse.Size(m)
}
func (m *ContactPingResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ContactPingResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ContactPingResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*CheckinRequest)(nil), "contact.CheckinRequest")
	proto.RegisterType((*CheckinResponse)(nil), "contact.CheckinResponse")
	proto.RegisterType((*ContactPingRequest)(nil), "contact.ContactPingRequest")
	proto.RegisterType((*ContactPingResponse)(nil), "contact.ContactPingResponse")
}

func init() { proto.RegisterFile("contact.proto", fileDescriptor_a5036fff2565fb15) }

var fileDescriptor_a5036fff2565fb15 = []byte{
	// 292 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x91, 0xb1, 0x4e, 0xc3, 0x30,
	0x10, 0x86, 0x95, 0x12, 0x91, 0x70, 0x08, 0x4a, 0x0d, 0x88, 0x28, 0x30, 0x54, 0x99, 0x2a, 0x40,
	0x19, 0xca, 0xca, 0x02, 0xa1, 0x23, 0x10, 0x99, 0x8d, 0x25, 0x72, 0x1d, 0x2b, 0x44, 0x15, 0xb6,
	0xb1, 0xdd, 0x81, 0x87, 0xe1, 0x5d, 0x91, 0x63, 0x27, 0x55, 0x29, 0x63, 0xff, 0xef, 0xbb, 0xbf,
	0x97, 0x33, 0x1c, 0x51, 0xc1, 0x0d, 0xa1, 0x26, 0x97, 0x4a, 0x18, 0x81, 0x22, 0xff, 0x33, 0x05,
	0x2e, 0x6a, 0xe6, 0xc2, 0xec, 0x27, 0x80, 0xe3, 0xe2, 0x83, 0xd1, 0x55, 0xcb, 0x31, 0xfb, 0x5a,
	0x33, 0x6d, 0xd0, 0x0d, 0x44, 0xa4, 0xae, 0x15, 0xd3, 0x3a, 0x09, 0xa6, 0xc1, 0xec, 0x70, 0x3e,
	0xc9, 0xbb, 0x81, 0x17, 0x51, 0xb3, 0x07, 0x07, 0x70, 0x6f, 0xa0, 0x1c, 0x62, 0x4a, 0x24, 0xa1,
	0xad, 0xf9, 0x4e, 0xf6, 0x3a, 0x1b, 0x6d, 0xec, 0xc2, 0x13, 0x3c, 0x38, 0xd6, 0x17, 0x92, 0x29,
	0x62, 0x84, 0x4a, 0xc2, 0xbf, 0xfe, 0xab, 0x27, 0x78, 0x70, 0xb2, 0x15, 0x8c, 0x87, 0xf5, 0xb4,
	0x14, 0x5c, 0x33, 0x74, 0x0d, 0x13, 0xd9, 0xf2, 0xa6, 0xb2, 0x63, 0x95, 0x5e, 0x53, 0xda, 0x6f,
	0x1a, 0xe3, 0xb1, 0x05, 0xb6, 0xe9, 0xcd, 0xc5, 0xe8, 0x16, 0x50, 0xe7, 0x32, 0xa5, 0x84, 0xaa,
	0x3e, 0x99, 0xd6, 0xa4, 0x61, 0xc9, 0x68, 0x1a, 0xcc, 0x0e, 0xf0, 0x89, 0x25, 0x0b, 0x0b, 0x9e,
	0x5d, 0x9e, 0x9d, 0x01, 0x2a, 0xdc, 0x8d, 0xca, 0x96, 0x37, 0xfe, 0x1e, 0xd9, 0x39, 0x9c, 0x6e,
	0xa5, 0x6e, 0x8d, 0x79, 0x09, 0x91, 0x8f, 0xd1, 0x02, 0xe2, 0xd2, 0xff, 0x31, 0xba, 0xcc, 0xfb,
	0xab, 0xef, 0x56, 0xa5, 0x57, 0xff, 0x43, 0xdf, 0xf8, 0x04, 0x61, 0x57, 0x71, 0x0f, 0x91, 0xff,
	0x66, 0x74, 0xb1, 0x19, 0xd8, 0x7a, 0xa4, 0x34, 0xd9, 0x05, 0xae, 0xe5, 0x31, 0x7c, 0x1f, 0xc9,
	0xe5, 0x72, 0xbf, 0x7b, 0xde, 0xbb, 0xdf, 0x00, 0x00, 0x00, 0xff, 0xff, 0xb5, 0xc7, 0x98, 0x61,
	0x04, 0x02, 0x00, 0x00,
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
	PingNode(ctx context.Context, in *ContactPingRequest, opts ...grpc.CallOption) (*ContactPingResponse, error)
}

type contactClient struct {
	cc *grpc.ClientConn
}

func NewContactClient(cc *grpc.ClientConn) ContactClient {
	return &contactClient{cc}
}

func (c *contactClient) PingNode(ctx context.Context, in *ContactPingRequest, opts ...grpc.CallOption) (*ContactPingResponse, error) {
	out := new(ContactPingResponse)
	err := c.cc.Invoke(ctx, "/contact.Contact/PingNode", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ContactServer is the server API for Contact service.
type ContactServer interface {
	PingNode(context.Context, *ContactPingRequest) (*ContactPingResponse, error)
}

func RegisterContactServer(s *grpc.Server, srv ContactServer) {
	s.RegisterService(&_Contact_serviceDesc, srv)
}

func _Contact_PingNode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ContactPingRequest)
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
		return srv.(ContactServer).PingNode(ctx, req.(*ContactPingRequest))
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
