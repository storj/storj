// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: crashreport.proto

package crashreportpb

import (
	fmt "fmt"
	math "math"

	proto "github.com/gogo/protobuf/proto"
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

type ReportRequest struct {
	GzippedPanic         []byte   `protobuf:"bytes,1,opt,name=gzipped_panic,json=gzippedPanic,proto3" json:"gzipped_panic,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReportRequest) Reset()         { *m = ReportRequest{} }
func (m *ReportRequest) String() string { return proto.CompactTextString(m) }
func (*ReportRequest) ProtoMessage()    {}
func (*ReportRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c640f4432300a07, []int{0}
}
func (m *ReportRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportRequest.Unmarshal(m, b)
}
func (m *ReportRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportRequest.Marshal(b, m, deterministic)
}
func (m *ReportRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportRequest.Merge(m, src)
}
func (m *ReportRequest) XXX_Size() int {
	return xxx_messageInfo_ReportRequest.Size(m)
}
func (m *ReportRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ReportRequest proto.InternalMessageInfo

func (m *ReportRequest) GetGzippedPanic() []byte {
	if m != nil {
		return m.GzippedPanic
	}
	return nil
}

type ReportResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ReportResponse) Reset()         { *m = ReportResponse{} }
func (m *ReportResponse) String() string { return proto.CompactTextString(m) }
func (*ReportResponse) ProtoMessage()    {}
func (*ReportResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_0c640f4432300a07, []int{1}
}
func (m *ReportResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ReportResponse.Unmarshal(m, b)
}
func (m *ReportResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ReportResponse.Marshal(b, m, deterministic)
}
func (m *ReportResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ReportResponse.Merge(m, src)
}
func (m *ReportResponse) XXX_Size() int {
	return xxx_messageInfo_ReportResponse.Size(m)
}
func (m *ReportResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ReportResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ReportResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*ReportRequest)(nil), "crash.ReportRequest")
	proto.RegisterType((*ReportResponse)(nil), "crash.ReportResponse")
}

func init() { proto.RegisterFile("crashreport.proto", fileDescriptor_0c640f4432300a07) }

var fileDescriptor_0c640f4432300a07 = []byte{
	// 166 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x4c, 0x2e, 0x4a, 0x2c,
	0xce, 0x28, 0x4a, 0x2d, 0xc8, 0x2f, 0x2a, 0xd1, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x05,
	0x0b, 0x29, 0x99, 0x70, 0xf1, 0x06, 0x81, 0x85, 0x83, 0x52, 0x0b, 0x4b, 0x53, 0x8b, 0x4b, 0x84,
	0x94, 0xb9, 0x78, 0xd3, 0xab, 0x32, 0x0b, 0x0a, 0x52, 0x53, 0xe2, 0x0b, 0x12, 0xf3, 0x32, 0x93,
	0x25, 0x18, 0x15, 0x18, 0x35, 0x78, 0x82, 0x78, 0xa0, 0x82, 0x01, 0x20, 0x31, 0x25, 0x01, 0x2e,
	0x3e, 0x98, 0xae, 0xe2, 0x82, 0xfc, 0xbc, 0xe2, 0x54, 0x23, 0x37, 0x2e, 0x6e, 0x67, 0x90, 0x81,
	0x10, 0x61, 0x21, 0x73, 0x2e, 0x36, 0x28, 0x4b, 0x44, 0x0f, 0x6c, 0x91, 0x1e, 0x8a, 0x2d, 0x52,
	0xa2, 0x68, 0xa2, 0x10, 0x53, 0x94, 0x18, 0x9c, 0xd4, 0xa2, 0x54, 0x8a, 0x4b, 0xf2, 0x8b, 0xb2,
	0xf4, 0x32, 0xf3, 0xf5, 0xc1, 0x0c, 0xfd, 0x82, 0xa2, 0xcc, 0xb2, 0xc4, 0x92, 0x54, 0x7d, 0x24,
	0x2f, 0x14, 0x24, 0x25, 0xb1, 0x81, 0x7d, 0x61, 0x0c, 0x08, 0x00, 0x00, 0xff, 0xff, 0x5b, 0xac,
	0xda, 0xa6, 0xda, 0x00, 0x00, 0x00,
}
