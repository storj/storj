// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: orders.proto

package pb

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"
import timestamp "github.com/golang/protobuf/ptypes/timestamp"

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
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

// PieceAction is an enumeration of all possible executed actions on storage node
type PieceAction int32

const (
	PieceAction_INVALID    PieceAction = 0
	PieceAction_PUT        PieceAction = 1
	PieceAction_GET        PieceAction = 2
	PieceAction_GET_AUDIT  PieceAction = 3
	PieceAction_GET_REPAIR PieceAction = 4
	PieceAction_PUT_REPAIR PieceAction = 5
	PieceAction_DELETE     PieceAction = 6
)

var PieceAction_name = map[int32]string{
	0: "INVALID",
	1: "PUT",
	2: "GET",
	3: "GET_AUDIT",
	4: "GET_REPAIR",
	5: "PUT_REPAIR",
	6: "DELETE",
}
var PieceAction_value = map[string]int32{
	"INVALID":    0,
	"PUT":        1,
	"GET":        2,
	"GET_AUDIT":  3,
	"GET_REPAIR": 4,
	"PUT_REPAIR": 5,
	"DELETE":     6,
}

func (x PieceAction) String() string {
	return proto.EnumName(PieceAction_name, int32(x))
}
func (PieceAction) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{0}
}

type SettlementResponse_Status int32

const (
	SettlementResponse_INVALID  SettlementResponse_Status = 0
	SettlementResponse_ACCEPTED SettlementResponse_Status = 1
	SettlementResponse_REJECTED SettlementResponse_Status = 2
)

var SettlementResponse_Status_name = map[int32]string{
	0: "INVALID",
	1: "ACCEPTED",
	2: "REJECTED",
}
var SettlementResponse_Status_value = map[string]int32{
	"INVALID":  0,
	"ACCEPTED": 1,
	"REJECTED": 2,
}

func (x SettlementResponse_Status) String() string {
	return proto.EnumName(SettlementResponse_Status_name, int32(x))
}
func (SettlementResponse_Status) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{4, 0}
}

// OrderLimit2 is provided by satellite to execute specific action on storage node within some limits
type OrderLimit2 struct {
	// unique serial to avoid replay attacks
	SerialNumber SerialNumber `protobuf:"bytes,1,opt,name=serial_number,json=serialNumber,proto3,customtype=SerialNumber" json:"serial_number"`
	// satellite who issued this order limit allowing orderer to do the specified action
	SatelliteId NodeID `protobuf:"bytes,2,opt,name=satellite_id,json=satelliteId,proto3,customtype=NodeID" json:"satellite_id"`
	// uplink who requested or whom behalf the order limit to do an action
	UplinkId NodeID `protobuf:"bytes,3,opt,name=uplink_id,json=uplinkId,proto3,customtype=NodeID" json:"uplink_id"`
	// storage node who can reclaim the order limit specified by serial
	StorageNodeId NodeID `protobuf:"bytes,4,opt,name=storage_node_id,json=storageNodeId,proto3,customtype=NodeID" json:"storage_node_id"`
	// piece which is allowed to be touched
	PieceId PieceID `protobuf:"bytes,5,opt,name=piece_id,json=pieceId,proto3,customtype=PieceID" json:"piece_id"`
	// limit in bytes how much can be changed
	Limit                int64                `protobuf:"varint,6,opt,name=limit,proto3" json:"limit,omitempty"`
	Action               PieceAction          `protobuf:"varint,7,opt,name=action,proto3,enum=orders.PieceAction" json:"action,omitempty"`
	PieceExpiration      *timestamp.Timestamp `protobuf:"bytes,8,opt,name=piece_expiration,json=pieceExpiration,proto3" json:"piece_expiration,omitempty"`
	OrderExpiration      *timestamp.Timestamp `protobuf:"bytes,9,opt,name=order_expiration,json=orderExpiration,proto3" json:"order_expiration,omitempty"`
	SatelliteSignature   []byte               `protobuf:"bytes,10,opt,name=satellite_signature,json=satelliteSignature,proto3" json:"satellite_signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *OrderLimit2) Reset()         { *m = OrderLimit2{} }
func (m *OrderLimit2) String() string { return proto.CompactTextString(m) }
func (*OrderLimit2) ProtoMessage()    {}
func (*OrderLimit2) Descriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{0}
}
func (m *OrderLimit2) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_OrderLimit2.Unmarshal(m, b)
}
func (m *OrderLimit2) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_OrderLimit2.Marshal(b, m, deterministic)
}
func (dst *OrderLimit2) XXX_Merge(src proto.Message) {
	xxx_messageInfo_OrderLimit2.Merge(dst, src)
}
func (m *OrderLimit2) XXX_Size() int {
	return xxx_messageInfo_OrderLimit2.Size(m)
}
func (m *OrderLimit2) XXX_DiscardUnknown() {
	xxx_messageInfo_OrderLimit2.DiscardUnknown(m)
}

var xxx_messageInfo_OrderLimit2 proto.InternalMessageInfo

func (m *OrderLimit2) GetLimit() int64 {
	if m != nil {
		return m.Limit
	}
	return 0
}

func (m *OrderLimit2) GetAction() PieceAction {
	if m != nil {
		return m.Action
	}
	return PieceAction_INVALID
}

func (m *OrderLimit2) GetPieceExpiration() *timestamp.Timestamp {
	if m != nil {
		return m.PieceExpiration
	}
	return nil
}

func (m *OrderLimit2) GetOrderExpiration() *timestamp.Timestamp {
	if m != nil {
		return m.OrderExpiration
	}
	return nil
}

func (m *OrderLimit2) GetSatelliteSignature() []byte {
	if m != nil {
		return m.SatelliteSignature
	}
	return nil
}

// Order2 is a one step of fullfilling Amount number of bytes from an OrderLimit2 with SerialNumber
type Order2 struct {
	// serial of the order limit that was signed
	SerialNumber SerialNumber `protobuf:"bytes,1,opt,name=serial_number,json=serialNumber,proto3,customtype=SerialNumber" json:"serial_number"`
	// amount to be signed for
	Amount int64 `protobuf:"varint,2,opt,name=amount,proto3" json:"amount,omitempty"`
	// signature
	UplinkSignature      []byte   `protobuf:"bytes,3,opt,name=uplink_signature,json=uplinkSignature,proto3" json:"uplink_signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Order2) Reset()         { *m = Order2{} }
func (m *Order2) String() string { return proto.CompactTextString(m) }
func (*Order2) ProtoMessage()    {}
func (*Order2) Descriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{1}
}
func (m *Order2) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Order2.Unmarshal(m, b)
}
func (m *Order2) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Order2.Marshal(b, m, deterministic)
}
func (dst *Order2) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Order2.Merge(dst, src)
}
func (m *Order2) XXX_Size() int {
	return xxx_messageInfo_Order2.Size(m)
}
func (m *Order2) XXX_DiscardUnknown() {
	xxx_messageInfo_Order2.DiscardUnknown(m)
}

var xxx_messageInfo_Order2 proto.InternalMessageInfo

func (m *Order2) GetAmount() int64 {
	if m != nil {
		return m.Amount
	}
	return 0
}

func (m *Order2) GetUplinkSignature() []byte {
	if m != nil {
		return m.UplinkSignature
	}
	return nil
}

type PieceHash struct {
	// piece id
	PieceId PieceID `protobuf:"bytes,1,opt,name=piece_id,json=pieceId,proto3,customtype=PieceID" json:"piece_id"`
	// hash of the piece that was/is uploaded
	Hash []byte `protobuf:"bytes,2,opt,name=hash,proto3" json:"hash,omitempty"`
	// signature either satellite or storage node
	Signature []byte `protobuf:"bytes,3,opt,name=signature,proto3" json:"signature,omitempty"`
	// size of uploaded piece
	PieceSize int64 `protobuf:"varint,4,opt,name=piece_size,json=pieceSize,proto3" json:"piece_size,omitempty"`
	// timestamp when upload occur
	Timestamp            *timestamp.Timestamp `protobuf:"bytes,5,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	XXX_NoUnkeyedLiteral struct{}             `json:"-"`
	XXX_unrecognized     []byte               `json:"-"`
	XXX_sizecache        int32                `json:"-"`
}

func (m *PieceHash) Reset()         { *m = PieceHash{} }
func (m *PieceHash) String() string { return proto.CompactTextString(m) }
func (*PieceHash) ProtoMessage()    {}
func (*PieceHash) Descriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{2}
}
func (m *PieceHash) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_PieceHash.Unmarshal(m, b)
}
func (m *PieceHash) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_PieceHash.Marshal(b, m, deterministic)
}
func (dst *PieceHash) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PieceHash.Merge(dst, src)
}
func (m *PieceHash) XXX_Size() int {
	return xxx_messageInfo_PieceHash.Size(m)
}
func (m *PieceHash) XXX_DiscardUnknown() {
	xxx_messageInfo_PieceHash.DiscardUnknown(m)
}

var xxx_messageInfo_PieceHash proto.InternalMessageInfo

func (m *PieceHash) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *PieceHash) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

func (m *PieceHash) GetPieceSize() int64 {
	if m != nil {
		return m.PieceSize
	}
	return 0
}

func (m *PieceHash) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

type SettlementRequest struct {
	Limit                *OrderLimit2 `protobuf:"bytes,1,opt,name=limit,proto3" json:"limit,omitempty"`
	Order                *Order2      `protobuf:"bytes,2,opt,name=order,proto3" json:"order,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *SettlementRequest) Reset()         { *m = SettlementRequest{} }
func (m *SettlementRequest) String() string { return proto.CompactTextString(m) }
func (*SettlementRequest) ProtoMessage()    {}
func (*SettlementRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{3}
}
func (m *SettlementRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SettlementRequest.Unmarshal(m, b)
}
func (m *SettlementRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SettlementRequest.Marshal(b, m, deterministic)
}
func (dst *SettlementRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SettlementRequest.Merge(dst, src)
}
func (m *SettlementRequest) XXX_Size() int {
	return xxx_messageInfo_SettlementRequest.Size(m)
}
func (m *SettlementRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_SettlementRequest.DiscardUnknown(m)
}

var xxx_messageInfo_SettlementRequest proto.InternalMessageInfo

func (m *SettlementRequest) GetLimit() *OrderLimit2 {
	if m != nil {
		return m.Limit
	}
	return nil
}

func (m *SettlementRequest) GetOrder() *Order2 {
	if m != nil {
		return m.Order
	}
	return nil
}

type SettlementResponse struct {
	SerialNumber         SerialNumber              `protobuf:"bytes,1,opt,name=serial_number,json=serialNumber,proto3,customtype=SerialNumber" json:"serial_number"`
	Status               SettlementResponse_Status `protobuf:"varint,2,opt,name=status,proto3,enum=orders.SettlementResponse_Status" json:"status,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                  `json:"-"`
	XXX_unrecognized     []byte                    `json:"-"`
	XXX_sizecache        int32                     `json:"-"`
}

func (m *SettlementResponse) Reset()         { *m = SettlementResponse{} }
func (m *SettlementResponse) String() string { return proto.CompactTextString(m) }
func (*SettlementResponse) ProtoMessage()    {}
func (*SettlementResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_orders_bf129bbee2b6e493, []int{4}
}
func (m *SettlementResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_SettlementResponse.Unmarshal(m, b)
}
func (m *SettlementResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_SettlementResponse.Marshal(b, m, deterministic)
}
func (dst *SettlementResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_SettlementResponse.Merge(dst, src)
}
func (m *SettlementResponse) XXX_Size() int {
	return xxx_messageInfo_SettlementResponse.Size(m)
}
func (m *SettlementResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_SettlementResponse.DiscardUnknown(m)
}

var xxx_messageInfo_SettlementResponse proto.InternalMessageInfo

func (m *SettlementResponse) GetStatus() SettlementResponse_Status {
	if m != nil {
		return m.Status
	}
	return SettlementResponse_INVALID
}

func init() {
	proto.RegisterType((*OrderLimit2)(nil), "orders.OrderLimit2")
	proto.RegisterType((*Order2)(nil), "orders.Order2")
	proto.RegisterType((*PieceHash)(nil), "orders.PieceHash")
	proto.RegisterType((*SettlementRequest)(nil), "orders.SettlementRequest")
	proto.RegisterType((*SettlementResponse)(nil), "orders.SettlementResponse")
	proto.RegisterEnum("orders.PieceAction", PieceAction_name, PieceAction_value)
	proto.RegisterEnum("orders.SettlementResponse_Status", SettlementResponse_Status_name, SettlementResponse_Status_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// OrdersClient is the client API for Orders service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type OrdersClient interface {
	Settlement(ctx context.Context, opts ...grpc.CallOption) (Orders_SettlementClient, error)
}

type ordersClient struct {
	cc *grpc.ClientConn
}

func NewOrdersClient(cc *grpc.ClientConn) OrdersClient {
	return &ordersClient{cc}
}

func (c *ordersClient) Settlement(ctx context.Context, opts ...grpc.CallOption) (Orders_SettlementClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Orders_serviceDesc.Streams[0], "/orders.Orders/Settlement", opts...)
	if err != nil {
		return nil, err
	}
	x := &ordersSettlementClient{stream}
	return x, nil
}

type Orders_SettlementClient interface {
	Send(*SettlementRequest) error
	Recv() (*SettlementResponse, error)
	grpc.ClientStream
}

type ordersSettlementClient struct {
	grpc.ClientStream
}

func (x *ordersSettlementClient) Send(m *SettlementRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *ordersSettlementClient) Recv() (*SettlementResponse, error) {
	m := new(SettlementResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// OrdersServer is the server API for Orders service.
type OrdersServer interface {
	Settlement(Orders_SettlementServer) error
}

func RegisterOrdersServer(s *grpc.Server, srv OrdersServer) {
	s.RegisterService(&_Orders_serviceDesc, srv)
}

func _Orders_Settlement_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(OrdersServer).Settlement(&ordersSettlementServer{stream})
}

type Orders_SettlementServer interface {
	Send(*SettlementResponse) error
	Recv() (*SettlementRequest, error)
	grpc.ServerStream
}

type ordersSettlementServer struct {
	grpc.ServerStream
}

func (x *ordersSettlementServer) Send(m *SettlementResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *ordersSettlementServer) Recv() (*SettlementRequest, error) {
	m := new(SettlementRequest)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Orders_serviceDesc = grpc.ServiceDesc{
	ServiceName: "orders.Orders",
	HandlerType: (*OrdersServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Settlement",
			Handler:       _Orders_Settlement_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "orders.proto",
}

func init() { proto.RegisterFile("orders.proto", fileDescriptor_orders_bf129bbee2b6e493) }

var fileDescriptor_orders_bf129bbee2b6e493 = []byte{
	// 666 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xa4, 0x53, 0x4d, 0x6f, 0xd3, 0x40,
	0x10, 0xed, 0xe6, 0xc3, 0x49, 0x26, 0x69, 0x62, 0xb6, 0x15, 0x0a, 0x11, 0xa8, 0x21, 0xe2, 0x10,
	0x5a, 0x29, 0xa5, 0x46, 0x42, 0xf4, 0x98, 0x36, 0x56, 0x31, 0xaa, 0x4a, 0xb4, 0x71, 0x39, 0x70,
	0x89, 0x9c, 0x7a, 0x71, 0x2d, 0x1c, 0xaf, 0xf1, 0xae, 0x25, 0xd4, 0x1f, 0xc0, 0x91, 0x7f, 0xc4,
	0x85, 0x13, 0xbf, 0x81, 0x43, 0x7f, 0x0b, 0xf2, 0xd8, 0xf9, 0x28, 0x14, 0x7a, 0xe8, 0xcd, 0x6f,
	0xe6, 0xbd, 0x19, 0xef, 0xbe, 0xb7, 0xd0, 0x10, 0xb1, 0xcb, 0x63, 0x39, 0x88, 0x62, 0xa1, 0x04,
	0xd5, 0x32, 0xd4, 0x01, 0x4f, 0x78, 0x22, 0xab, 0x75, 0x76, 0x3c, 0x21, 0xbc, 0x80, 0xef, 0x23,
	0x9a, 0x25, 0x1f, 0xf7, 0x95, 0x3f, 0xe7, 0x52, 0x39, 0xf3, 0x28, 0x23, 0xf4, 0xbe, 0x95, 0xa0,
	0xfe, 0x2e, 0xd5, 0x9d, 0xfa, 0x73, 0x5f, 0x19, 0xf4, 0x10, 0x36, 0x25, 0x8f, 0x7d, 0x27, 0x98,
	0x86, 0xc9, 0x7c, 0xc6, 0xe3, 0x36, 0xe9, 0x92, 0x7e, 0xe3, 0x68, 0xfb, 0xe7, 0xf5, 0xce, 0xc6,
	0xaf, 0xeb, 0x9d, 0xc6, 0x04, 0x9b, 0x67, 0xd8, 0x63, 0x0d, 0xb9, 0x86, 0xe8, 0x01, 0x34, 0xa4,
	0xa3, 0x78, 0x10, 0xf8, 0x8a, 0x4f, 0x7d, 0xb7, 0x5d, 0x40, 0x65, 0x33, 0x57, 0x6a, 0x67, 0xc2,
	0xe5, 0xd6, 0x88, 0xd5, 0x97, 0x1c, 0xcb, 0xa5, 0x7b, 0x50, 0x4b, 0xa2, 0xc0, 0x0f, 0x3f, 0xa5,
	0xfc, 0xe2, 0xad, 0xfc, 0x6a, 0x46, 0xb0, 0x5c, 0xfa, 0x0a, 0x5a, 0x52, 0x89, 0xd8, 0xf1, 0xf8,
	0x34, 0x14, 0x2e, 0xae, 0x28, 0xdd, 0x2a, 0xd9, 0xcc, 0x69, 0x08, 0x5d, 0xba, 0x0b, 0xd5, 0xc8,
	0xe7, 0x17, 0x28, 0x28, 0xa3, 0xa0, 0x95, 0x0b, 0x2a, 0xe3, 0xb4, 0x6e, 0x8d, 0x58, 0x05, 0x09,
	0x96, 0x4b, 0xb7, 0xa1, 0x1c, 0xa4, 0x17, 0xd1, 0xd6, 0xba, 0xa4, 0x5f, 0x64, 0x19, 0xa0, 0x7b,
	0xa0, 0x39, 0x17, 0xca, 0x17, 0x61, 0xbb, 0xd2, 0x25, 0xfd, 0xa6, 0xb1, 0x35, 0xc8, 0x2f, 0x1e,
	0xf5, 0x43, 0x6c, 0xb1, 0x9c, 0x42, 0x4d, 0xd0, 0xb3, 0x75, 0xfc, 0x4b, 0xe4, 0xc7, 0x0e, 0xca,
	0xaa, 0x5d, 0xd2, 0xaf, 0x1b, 0x9d, 0x41, 0xe6, 0xc6, 0x60, 0xe1, 0xc6, 0xc0, 0x5e, 0xb8, 0xc1,
	0x5a, 0xa8, 0x31, 0x97, 0x92, 0x74, 0x0c, 0x2e, 0x59, 0x1f, 0x53, 0xbb, 0x7b, 0x0c, 0x6a, 0xd6,
	0xc6, 0xec, 0xc3, 0xd6, 0xca, 0x14, 0xe9, 0x7b, 0xa1, 0xa3, 0x92, 0x98, 0xb7, 0x21, 0xbd, 0x07,
	0x46, 0x97, 0xad, 0xc9, 0xa2, 0xd3, 0xfb, 0x4a, 0x40, 0xc3, 0x40, 0xdc, 0x2b, 0x0b, 0x0f, 0x41,
	0x73, 0xe6, 0x22, 0x09, 0x15, 0xa6, 0xa0, 0xc8, 0x72, 0x44, 0x9f, 0x83, 0x9e, 0x1b, 0xbe, 0xfa,
	0x17, 0xf4, 0x9d, 0xb5, 0xb2, 0xfa, 0xea, 0x47, 0x7e, 0x10, 0xa8, 0xe1, 0xfd, 0xbe, 0x71, 0xe4,
	0xe5, 0x0d, 0x13, 0xc9, 0x1d, 0x26, 0x52, 0x28, 0x5d, 0x3a, 0xf2, 0x32, 0x0b, 0x20, 0xc3, 0x6f,
	0xfa, 0x18, 0x6a, 0x7f, 0x6e, 0x5c, 0x15, 0xe8, 0x13, 0x80, 0x6c, 0xba, 0xf4, 0xaf, 0x38, 0xa6,
	0xaa, 0xc8, 0x6a, 0x58, 0x99, 0xf8, 0x57, 0x9c, 0xbe, 0x86, 0xda, 0xf2, 0xdd, 0x60, 0x84, 0xfe,
	0x6f, 0xc2, 0x8a, 0xdc, 0x73, 0xe1, 0xc1, 0x84, 0x2b, 0x15, 0xf0, 0x39, 0x0f, 0x15, 0xe3, 0x9f,
	0x13, 0x2e, 0xd3, 0x4b, 0xc8, 0x43, 0x46, 0x70, 0xd4, 0x32, 0x4d, 0x6b, 0xef, 0x70, 0x91, 0xbc,
	0x67, 0x50, 0xc6, 0x26, 0x9e, 0xa5, 0x6e, 0x34, 0x6f, 0x50, 0x0d, 0x96, 0x35, 0x7b, 0xdf, 0x09,
	0xd0, 0xf5, 0x35, 0x32, 0x12, 0xa1, 0xe4, 0xf7, 0xf1, 0xef, 0x10, 0x34, 0xa9, 0x1c, 0x95, 0x48,
	0x5c, 0xdc, 0x34, 0x9e, 0x2e, 0x16, 0xff, 0xbd, 0x66, 0x30, 0x41, 0x22, 0xcb, 0x05, 0xbd, 0x03,
	0xd0, 0xb2, 0x0a, 0xad, 0x43, 0xc5, 0x3a, 0x7b, 0x3f, 0x3c, 0xb5, 0x46, 0xfa, 0x06, 0x6d, 0x40,
	0x75, 0x78, 0x7c, 0x6c, 0x8e, 0x6d, 0x73, 0xa4, 0x93, 0x14, 0x31, 0xf3, 0xad, 0x79, 0x9c, 0xa2,
	0xc2, 0xae, 0x07, 0xf5, 0xb5, 0x97, 0x74, 0x53, 0x57, 0x81, 0xe2, 0xf8, 0xdc, 0xd6, 0x49, 0xfa,
	0x71, 0x62, 0xda, 0x7a, 0x81, 0x6e, 0x42, 0xed, 0xc4, 0xb4, 0xa7, 0xc3, 0xf3, 0x91, 0x65, 0xeb,
	0x45, 0xda, 0x04, 0x48, 0x21, 0x33, 0xc7, 0x43, 0x8b, 0xe9, 0xa5, 0x14, 0x8f, 0xcf, 0x97, 0xb8,
	0x4c, 0x01, 0xb4, 0x91, 0x79, 0x6a, 0xda, 0xa6, 0xae, 0x19, 0x93, 0x3c, 0xdb, 0x92, 0x5a, 0x00,
	0xab, 0xa3, 0xd0, 0x47, 0xb7, 0x1d, 0x0f, 0xcd, 0xea, 0x74, 0xfe, 0x7d, 0xf2, 0xde, 0x46, 0x9f,
	0xbc, 0x20, 0x47, 0xa5, 0x0f, 0x85, 0x68, 0x36, 0xd3, 0x30, 0x08, 0x2f, 0x7f, 0x07, 0x00, 0x00,
	0xff, 0xff, 0xda, 0xd3, 0x2a, 0x0e, 0x94, 0x05, 0x00, 0x00,
}
