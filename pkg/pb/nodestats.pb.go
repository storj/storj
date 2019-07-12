// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: nodestats.proto

package pb

import (
	context "context"
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	_ "github.com/golang/protobuf/ptypes/timestamp"
	grpc "google.golang.org/grpc"
	math "math"
	time "time"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf
var _ = time.Kitchen

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type ReputationStats struct {
	TotalCount           int64    `protobuf:"varint,1,opt,name=total_count,json=totalCount,proto3" json:"total_count,omitempty"`
	SuccessCount         int64    `protobuf:"varint,2,opt,name=success_count,json=successCount,proto3" json:"success_count,omitempty"`
	ReputationAlpha      float64  `protobuf:"fixed64,3,opt,name=reputation_alpha,json=reputationAlpha,proto3" json:"reputation_alpha,omitempty"`
	ReputationBeta       float64  `protobuf:"fixed64,4,opt,name=reputation_beta,json=reputationBeta,proto3" json:"reputation_beta,omitempty"`
	ReputationScore      float64  `protobuf:"fixed64,5,opt,name=reputation_score,json=reputationScore,proto3" json:"reputation_score,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReputationStats) Reset()         { *m = ReputationStats{} }
func (m *ReputationStats) String() string { return proto.CompactTextString(m) }
func (*ReputationStats) ProtoMessage()    {}
func (*ReputationStats) Descriptor() ([]byte, []int) {
	return fileDescriptor_e0b184ee117142aa, []int{0}
}
func (m *ReputationStats) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReputationStats.Unmarshal(m, b)
}
func (m *ReputationStats) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReputationStats.Marshal(b, m, deterministic)
}
func (m *ReputationStats) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReputationStats.Merge(m, src)
}
func (m *ReputationStats) XXX_Size() int {
	return xxx_messageInfo_ReputationStats.Size(m)
}
func (m *ReputationStats) XXX_DiscardUnknown() {
	xxx_messageInfo_ReputationStats.DiscardUnknown(m)
}

var xxx_messageInfo_ReputationStats proto.InternalMessageInfo

func (m *ReputationStats) GetTotalCount() int64 {
	if m != nil {
		return m.TotalCount
	}
	return 0
}

func (m *ReputationStats) GetSuccessCount() int64 {
	if m != nil {
		return m.SuccessCount
	}
	return 0
}

func (m *ReputationStats) GetReputationAlpha() float64 {
	if m != nil {
		return m.ReputationAlpha
	}
	return 0
}

func (m *ReputationStats) GetReputationBeta() float64 {
	if m != nil {
		return m.ReputationBeta
	}
	return 0
}

func (m *ReputationStats) GetReputationScore() float64 {
	if m != nil {
		return m.ReputationScore
	}
	return 0
}

type GetStatsRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *GetStatsRequest) Reset()         { *m = GetStatsRequest{} }
func (m *GetStatsRequest) String() string { return proto.CompactTextString(m) }
func (*GetStatsRequest) ProtoMessage()    {}
func (*GetStatsRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_e0b184ee117142aa, []int{1}
}
func (m *GetStatsRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetStatsRequest.Unmarshal(m, b)
}
func (m *GetStatsRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetStatsRequest.Marshal(b, m, deterministic)
}
func (m *GetStatsRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetStatsRequest.Merge(m, src)
}
func (m *GetStatsRequest) XXX_Size() int {
	return xxx_messageInfo_GetStatsRequest.Size(m)
}
func (m *GetStatsRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GetStatsRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GetStatsRequest proto.InternalMessageInfo

type GetStatsResponse struct {
	UptimeCheck          *ReputationStats `protobuf:"bytes,1,opt,name=uptime_check,json=uptimeCheck,proto3" json:"uptime_check,omitempty"`
	AuditCheck           *ReputationStats `protobuf:"bytes,2,opt,name=audit_check,json=auditCheck,proto3" json:"audit_check,omitempty"`
	TimeStamp            time.Time        `protobuf:"bytes,3,opt,name=time_stamp,json=timeStamp,proto3,stdtime" json:"time_stamp"`
	XXX_NoUnkeyedLiteral struct{}         `json:"-"`
	XXX_unrecognized     []byte           `json:"-"`
	XXX_sizecache        int32            `json:"-"`
}

func (m *GetStatsResponse) Reset()         { *m = GetStatsResponse{} }
func (m *GetStatsResponse) String() string { return proto.CompactTextString(m) }
func (*GetStatsResponse) ProtoMessage()    {}
func (*GetStatsResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_e0b184ee117142aa, []int{2}
}
func (m *GetStatsResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_GetStatsResponse.Unmarshal(m, b)
}
func (m *GetStatsResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_GetStatsResponse.Marshal(b, m, deterministic)
}
func (m *GetStatsResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetStatsResponse.Merge(m, src)
}
func (m *GetStatsResponse) XXX_Size() int {
	return xxx_messageInfo_GetStatsResponse.Size(m)
}
func (m *GetStatsResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GetStatsResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GetStatsResponse proto.InternalMessageInfo

func (m *GetStatsResponse) GetUptimeCheck() *ReputationStats {
	if m != nil {
		return m.UptimeCheck
	}
	return nil
}

func (m *GetStatsResponse) GetAuditCheck() *ReputationStats {
	if m != nil {
		return m.AuditCheck
	}
	return nil
}

func (m *GetStatsResponse) GetTimeStamp() time.Time {
	if m != nil {
		return m.TimeStamp
	}
	return time.Time{}
}

type DailyStorageUsageRequest struct {
	From                 time.Time `protobuf:"bytes,1,opt,name=from,proto3,stdtime" json:"from"`
	To                   time.Time `protobuf:"bytes,2,opt,name=to,proto3,stdtime" json:"to"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *DailyStorageUsageRequest) Reset()         { *m = DailyStorageUsageRequest{} }
func (m *DailyStorageUsageRequest) String() string { return proto.CompactTextString(m) }
func (*DailyStorageUsageRequest) ProtoMessage()    {}
func (*DailyStorageUsageRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_e0b184ee117142aa, []int{3}
}
func (m *DailyStorageUsageRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DailyStorageUsageRequest.Unmarshal(m, b)
}
func (m *DailyStorageUsageRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DailyStorageUsageRequest.Marshal(b, m, deterministic)
}
func (m *DailyStorageUsageRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DailyStorageUsageRequest.Merge(m, src)
}
func (m *DailyStorageUsageRequest) XXX_Size() int {
	return xxx_messageInfo_DailyStorageUsageRequest.Size(m)
}
func (m *DailyStorageUsageRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_DailyStorageUsageRequest.DiscardUnknown(m)
}

var xxx_messageInfo_DailyStorageUsageRequest proto.InternalMessageInfo

func (m *DailyStorageUsageRequest) GetFrom() time.Time {
	if m != nil {
		return m.From
	}
	return time.Time{}
}

func (m *DailyStorageUsageRequest) GetTo() time.Time {
	if m != nil {
		return m.To
	}
	return time.Time{}
}

type DailyStorageUsageResponse struct {
	NodeId               NodeID                                    `protobuf:"bytes,1,opt,name=node_id,json=nodeId,proto3,customtype=NodeID" json:"node_id"`
	DailyStorageUsage    []*DailyStorageUsageResponse_StorageUsage `protobuf:"bytes,2,rep,name=daily_storage_usage,json=dailyStorageUsage,proto3" json:"daily_storage_usage,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                                  `json:"-"`
	XXX_unrecognized     []byte                                    `json:"-"`
	XXX_sizecache        int32                                     `json:"-"`
}

func (m *DailyStorageUsageResponse) Reset()         { *m = DailyStorageUsageResponse{} }
func (m *DailyStorageUsageResponse) String() string { return proto.CompactTextString(m) }
func (*DailyStorageUsageResponse) ProtoMessage()    {}
func (*DailyStorageUsageResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_e0b184ee117142aa, []int{4}
}
func (m *DailyStorageUsageResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DailyStorageUsageResponse.Unmarshal(m, b)
}
func (m *DailyStorageUsageResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DailyStorageUsageResponse.Marshal(b, m, deterministic)
}
func (m *DailyStorageUsageResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DailyStorageUsageResponse.Merge(m, src)
}
func (m *DailyStorageUsageResponse) XXX_Size() int {
	return xxx_messageInfo_DailyStorageUsageResponse.Size(m)
}
func (m *DailyStorageUsageResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_DailyStorageUsageResponse.DiscardUnknown(m)
}

var xxx_messageInfo_DailyStorageUsageResponse proto.InternalMessageInfo

func (m *DailyStorageUsageResponse) GetDailyStorageUsage() []*DailyStorageUsageResponse_StorageUsage {
	if m != nil {
		return m.DailyStorageUsage
	}
	return nil
}

type DailyStorageUsageResponse_StorageUsage struct {
	RollupId             int64     `protobuf:"varint,1,opt,name=rollup_id,json=rollupId,proto3" json:"rollup_id,omitempty"`
	AtRestTotal          float64   `protobuf:"fixed64,2,opt,name=at_rest_total,json=atRestTotal,proto3" json:"at_rest_total,omitempty"`
	TimeStamp            time.Time `protobuf:"bytes,3,opt,name=time_stamp,json=timeStamp,proto3,stdtime" json:"time_stamp"`
	XXX_NoUnkeyedLiteral struct{}  `json:"-"`
	XXX_unrecognized     []byte    `json:"-"`
	XXX_sizecache        int32     `json:"-"`
}

func (m *DailyStorageUsageResponse_StorageUsage) Reset() {
	*m = DailyStorageUsageResponse_StorageUsage{}
}
func (m *DailyStorageUsageResponse_StorageUsage) String() string { return proto.CompactTextString(m) }
func (*DailyStorageUsageResponse_StorageUsage) ProtoMessage()    {}
func (*DailyStorageUsageResponse_StorageUsage) Descriptor() ([]byte, []int) {
	return fileDescriptor_e0b184ee117142aa, []int{4, 0}
}
func (m *DailyStorageUsageResponse_StorageUsage) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_DailyStorageUsageResponse_StorageUsage.Unmarshal(m, b)
}
func (m *DailyStorageUsageResponse_StorageUsage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_DailyStorageUsageResponse_StorageUsage.Marshal(b, m, deterministic)
}
func (m *DailyStorageUsageResponse_StorageUsage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_DailyStorageUsageResponse_StorageUsage.Merge(m, src)
}
func (m *DailyStorageUsageResponse_StorageUsage) XXX_Size() int {
	return xxx_messageInfo_DailyStorageUsageResponse_StorageUsage.Size(m)
}
func (m *DailyStorageUsageResponse_StorageUsage) XXX_DiscardUnknown() {
	xxx_messageInfo_DailyStorageUsageResponse_StorageUsage.DiscardUnknown(m)
}

var xxx_messageInfo_DailyStorageUsageResponse_StorageUsage proto.InternalMessageInfo

func (m *DailyStorageUsageResponse_StorageUsage) GetRollupId() int64 {
	if m != nil {
		return m.RollupId
	}
	return 0
}

func (m *DailyStorageUsageResponse_StorageUsage) GetAtRestTotal() float64 {
	if m != nil {
		return m.AtRestTotal
	}
	return 0
}

func (m *DailyStorageUsageResponse_StorageUsage) GetTimeStamp() time.Time {
	if m != nil {
		return m.TimeStamp
	}
	return time.Time{}
}

func init() {
	proto.RegisterType((*ReputationStats)(nil), "nodestats.ReputationStats")
	proto.RegisterType((*GetStatsRequest)(nil), "nodestats.GetStatsRequest")
	proto.RegisterType((*GetStatsResponse)(nil), "nodestats.GetStatsResponse")
	proto.RegisterType((*DailyStorageUsageRequest)(nil), "nodestats.DailyStorageUsageRequest")
	proto.RegisterType((*DailyStorageUsageResponse)(nil), "nodestats.DailyStorageUsageResponse")
	proto.RegisterType((*DailyStorageUsageResponse_StorageUsage)(nil), "nodestats.DailyStorageUsageResponse.StorageUsage")
}

func init() { proto.RegisterFile("nodestats.proto", fileDescriptor_e0b184ee117142aa) }

var fileDescriptor_e0b184ee117142aa = []byte{
	// 530 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xac, 0x93, 0xbf, 0x8e, 0xd3, 0x4c,
	0x14, 0xc5, 0xbf, 0x71, 0xf2, 0x85, 0xe4, 0x3a, 0xbb, 0xd9, 0x0c, 0x8d, 0xc9, 0x16, 0x89, 0xbc,
	0x48, 0x1b, 0x9a, 0xac, 0x08, 0x14, 0x48, 0x88, 0x82, 0x64, 0x25, 0x94, 0x86, 0x62, 0xb2, 0x34,
	0x14, 0x58, 0x13, 0x7b, 0xd6, 0x6b, 0xe1, 0x64, 0x8c, 0xe7, 0xba, 0xe0, 0x15, 0xa8, 0x28, 0x78,
	0x10, 0x9e, 0x80, 0x9a, 0x1e, 0x89, 0x82, 0x62, 0x79, 0x15, 0x34, 0x33, 0xce, 0x1f, 0xcc, 0x02,
	0x29, 0x28, 0xfd, 0xf3, 0x39, 0xc7, 0x73, 0xcf, 0x1d, 0x43, 0x67, 0x25, 0x23, 0xa1, 0x90, 0xa3,
	0x1a, 0x65, 0xb9, 0x44, 0x49, 0x5b, 0x1b, 0xd0, 0x83, 0x58, 0xc6, 0xd2, 0xe2, 0x5e, 0x3f, 0x96,
	0x32, 0x4e, 0xc5, 0x99, 0x79, 0x5a, 0x14, 0x97, 0x67, 0x98, 0x2c, 0xb5, 0x6c, 0x99, 0x59, 0x81,
	0xff, 0x95, 0x40, 0x87, 0x89, 0xac, 0x40, 0x8e, 0x89, 0x5c, 0xcd, 0x75, 0x00, 0xed, 0x83, 0x8b,
	0x12, 0x79, 0x1a, 0x84, 0xb2, 0x58, 0xa1, 0x47, 0x06, 0x64, 0x58, 0x63, 0x60, 0xd0, 0x54, 0x13,
	0x7a, 0x02, 0x07, 0xaa, 0x08, 0x43, 0xa1, 0x54, 0x29, 0x71, 0x8c, 0xa4, 0x5d, 0x42, 0x2b, 0xba,
	0x07, 0x47, 0xf9, 0x26, 0x38, 0xe0, 0x69, 0x76, 0xc5, 0xbd, 0xda, 0x80, 0x0c, 0x09, 0xeb, 0x6c,
	0xf9, 0x53, 0x8d, 0xe9, 0x29, 0xec, 0xa0, 0x60, 0x21, 0x90, 0x7b, 0x75, 0xa3, 0x3c, 0xdc, 0xe2,
	0x89, 0x40, 0x5e, 0xc9, 0x54, 0xa1, 0xcc, 0x85, 0xf7, 0x7f, 0x35, 0x73, 0xae, 0xb1, 0xdf, 0x85,
	0xce, 0x33, 0x81, 0x66, 0x20, 0x26, 0xde, 0x14, 0x42, 0xa1, 0xff, 0x85, 0xc0, 0xd1, 0x96, 0xa9,
	0x4c, 0xae, 0x94, 0xa0, 0x4f, 0xa0, 0x5d, 0x64, 0xba, 0x95, 0x20, 0xbc, 0x12, 0xe1, 0x6b, 0x33,
	0xad, 0x3b, 0xee, 0x8d, 0xb6, 0x05, 0x57, 0xea, 0x61, 0xae, 0xd5, 0x4f, 0xb5, 0x9c, 0x3e, 0x06,
	0x97, 0x17, 0x51, 0x82, 0xa5, 0xdb, 0xf9, 0xab, 0x1b, 0x8c, 0xdc, 0x9a, 0xa7, 0x00, 0xe6, 0xcb,
	0x66, 0x21, 0xa6, 0x1c, 0xed, 0xb5, 0x2b, 0x1b, 0xad, 0x57, 0x36, 0xba, 0x58, 0xaf, 0x6c, 0xd2,
	0xfc, 0x7c, 0xdd, 0xff, 0xef, 0xfd, 0xf7, 0x3e, 0x61, 0x2d, 0xed, 0x9b, 0x6b, 0xe8, 0xbf, 0x23,
	0xe0, 0x9d, 0xf3, 0x24, 0x7d, 0x3b, 0x47, 0x99, 0xf3, 0x58, 0xbc, 0x50, 0x3c, 0x16, 0xe5, 0xc8,
	0xf4, 0x11, 0xd4, 0x2f, 0x73, 0xb9, 0xdc, 0x4c, 0xb5, 0x4f, 0xb6, 0x71, 0xd0, 0x87, 0xe0, 0xa0,
	0xdc, 0xcc, 0xb3, 0x8f, 0xcf, 0x41, 0xe9, 0x7f, 0x72, 0xe0, 0xce, 0x0d, 0x87, 0x29, 0xbb, 0x3e,
	0x85, 0x5b, 0xba, 0x98, 0x20, 0x89, 0xcc, 0x81, 0xda, 0x93, 0x43, 0x6d, 0xfe, 0x76, 0xdd, 0x6f,
	0x3c, 0x97, 0x91, 0x98, 0x9d, 0xb3, 0x86, 0x7e, 0x3d, 0x8b, 0x28, 0x87, 0xdb, 0x91, 0x4e, 0x09,
	0x94, 0x8d, 0x09, 0x0a, 0x9d, 0xe3, 0x39, 0x83, 0xda, 0xd0, 0x1d, 0xdf, 0xdf, 0x69, 0xf7, 0xb7,
	0xdf, 0x1a, 0xfd, 0x04, 0xbb, 0x51, 0x55, 0xd7, 0xfb, 0x40, 0xa0, 0xbd, 0x0b, 0xe8, 0x31, 0xb4,
	0x72, 0x99, 0xa6, 0x45, 0xb6, 0x3e, 0x5e, 0x8d, 0x35, 0x2d, 0x98, 0x45, 0xd4, 0x87, 0x03, 0x8e,
	0x41, 0x2e, 0x14, 0x06, 0xe6, 0x3f, 0x30, 0xc5, 0x10, 0xe6, 0x72, 0x64, 0x42, 0xe1, 0x85, 0x46,
	0xff, 0x64, 0x9b, 0xe3, 0x8f, 0x04, 0x5a, 0xba, 0x0c, 0xfb, 0x27, 0x4e, 0xa1, 0xb9, 0xbe, 0xb0,
	0x74, 0xf7, 0x52, 0x55, 0x6e, 0x76, 0xef, 0xf8, 0xc6, 0x77, 0x65, 0xeb, 0xaf, 0xa0, 0xfb, 0x4b,
	0x4d, 0xf4, 0xe4, 0xcf, 0x25, 0xda, 0xd8, 0xbb, 0xfb, 0x34, 0x3d, 0xa9, 0xbf, 0x74, 0xb2, 0xc5,
	0xa2, 0x61, 0x26, 0x7c, 0xf0, 0x23, 0x00, 0x00, 0xff, 0xff, 0x33, 0x26, 0xdd, 0xea, 0x9a, 0x04,
	0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// NodeStatsClient is the client API for NodeStats service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type NodeStatsClient interface {
	GetStats(ctx context.Context, in *GetStatsRequest, opts ...grpc.CallOption) (*GetStatsResponse, error)
	DailyStorageUsage(ctx context.Context, in *DailyStorageUsageRequest, opts ...grpc.CallOption) (*DailyStorageUsageResponse, error)
}

type nodeStatsClient struct {
	cc *grpc.ClientConn
}

func NewNodeStatsClient(cc *grpc.ClientConn) NodeStatsClient {
	return &nodeStatsClient{cc}
}

func (c *nodeStatsClient) GetStats(ctx context.Context, in *GetStatsRequest, opts ...grpc.CallOption) (*GetStatsResponse, error) {
	out := new(GetStatsResponse)
	err := c.cc.Invoke(ctx, "/nodestats.NodeStats/GetStats", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *nodeStatsClient) DailyStorageUsage(ctx context.Context, in *DailyStorageUsageRequest, opts ...grpc.CallOption) (*DailyStorageUsageResponse, error) {
	out := new(DailyStorageUsageResponse)
	err := c.cc.Invoke(ctx, "/nodestats.NodeStats/DailyStorageUsage", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// NodeStatsServer is the server API for NodeStats service.
type NodeStatsServer interface {
	GetStats(context.Context, *GetStatsRequest) (*GetStatsResponse, error)
	DailyStorageUsage(context.Context, *DailyStorageUsageRequest) (*DailyStorageUsageResponse, error)
}

func RegisterNodeStatsServer(s *grpc.Server, srv NodeStatsServer) {
	s.RegisterService(&_NodeStats_serviceDesc, srv)
}

func _NodeStats_GetStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeStatsServer).GetStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodestats.NodeStats/GetStats",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeStatsServer).GetStats(ctx, req.(*GetStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _NodeStats_DailyStorageUsage_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DailyStorageUsageRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(NodeStatsServer).DailyStorageUsage(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/nodestats.NodeStats/DailyStorageUsage",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(NodeStatsServer).DailyStorageUsage(ctx, req.(*DailyStorageUsageRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _NodeStats_serviceDesc = grpc.ServiceDesc{
	ServiceName: "nodestats.NodeStats",
	HandlerType: (*NodeStatsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetStats",
			Handler:    _NodeStats_GetStats_Handler,
		},
		{
			MethodName: "DailyStorageUsage",
			Handler:    _NodeStats_DailyStorageUsage_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "nodestats.proto",
}
