// Code generated by protoc-gen-go. DO NOT EDIT.
// source: meta.proto

package streams

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type MetaStreamInfo struct {
	NumberOfSegments     int64    `protobuf:"varint,1,opt,name=number_of_segments,json=numberOfSegments" json:"number_of_segments,omitempty"`
	SegmentsSize         int64    `protobuf:"varint,2,opt,name=segments_size,json=segmentsSize" json:"segments_size,omitempty"`
	LastSegmentSize      int64    `protobuf:"varint,3,opt,name=last_segment_size,json=lastSegmentSize" json:"last_segment_size,omitempty"`
	Metadata             []byte   `protobuf:"bytes,4,opt,name=metadata,proto3" json:"metadata,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *MetaStreamInfo) Reset()         { *m = MetaStreamInfo{} }
func (m *MetaStreamInfo) String() string { return proto.CompactTextString(m) }
func (*MetaStreamInfo) ProtoMessage()    {}
func (*MetaStreamInfo) Descriptor() ([]byte, []int) {
	return fileDescriptor_meta_1e6a51dbfd2db316, []int{0}
}
func (m *MetaStreamInfo) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_MetaStreamInfo.Unmarshal(m, b)
}
func (m *MetaStreamInfo) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_MetaStreamInfo.Marshal(b, m, deterministic)
}
func (dst *MetaStreamInfo) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MetaStreamInfo.Merge(dst, src)
}
func (m *MetaStreamInfo) XXX_Size() int {
	return xxx_messageInfo_MetaStreamInfo.Size(m)
}
func (m *MetaStreamInfo) XXX_DiscardUnknown() {
	xxx_messageInfo_MetaStreamInfo.DiscardUnknown(m)
}

var xxx_messageInfo_MetaStreamInfo proto.InternalMessageInfo

func (m *MetaStreamInfo) GetNumberOfSegments() int64 {
	if m != nil {
		return m.NumberOfSegments
	}
	return 0
}

func (m *MetaStreamInfo) GetSegmentsSize() int64 {
	if m != nil {
		return m.SegmentsSize
	}
	return 0
}

func (m *MetaStreamInfo) GetLastSegmentSize() int64 {
	if m != nil {
		return m.LastSegmentSize
	}
	return 0
}

func (m *MetaStreamInfo) GetMetadata() []byte {
	if m != nil {
		return m.Metadata
	}
	return nil
}

func init() {
	proto.RegisterType((*MetaStreamInfo)(nil), "streams.MetaStreamInfo")
}

func init() { proto.RegisterFile("meta.proto", fileDescriptor_meta_1e6a51dbfd2db316) }

var fileDescriptor_meta_1e6a51dbfd2db316 = []byte{
	// 165 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0xca, 0x4d, 0x2d, 0x49,
	0xd4, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x2f, 0x2e, 0x29, 0x4a, 0x4d, 0xcc, 0x2d, 0x56,
	0x5a, 0xcd, 0xc8, 0xc5, 0xe7, 0x9b, 0x5a, 0x92, 0x18, 0x0c, 0xe6, 0x7b, 0xe6, 0xa5, 0xe5, 0x0b,
	0xe9, 0x70, 0x09, 0xe5, 0x95, 0xe6, 0x26, 0xa5, 0x16, 0xc5, 0xe7, 0xa7, 0xc5, 0x17, 0xa7, 0xa6,
	0xe7, 0xa6, 0xe6, 0x95, 0x14, 0x4b, 0x30, 0x2a, 0x30, 0x6a, 0x30, 0x07, 0x09, 0x40, 0x64, 0xfc,
	0xd3, 0x82, 0xa1, 0xe2, 0x42, 0xca, 0x5c, 0xbc, 0x30, 0x35, 0xf1, 0xc5, 0x99, 0x55, 0xa9, 0x12,
	0x4c, 0x60, 0x85, 0x3c, 0x30, 0xc1, 0xe0, 0xcc, 0xaa, 0x54, 0x21, 0x2d, 0x2e, 0xc1, 0x9c, 0xc4,
	0xe2, 0x12, 0x98, 0x69, 0x10, 0x85, 0xcc, 0x60, 0x85, 0xfc, 0x20, 0x09, 0xa8, 0x69, 0x60, 0xb5,
	0x52, 0x5c, 0x1c, 0x20, 0x87, 0xa6, 0x24, 0x96, 0x24, 0x4a, 0xb0, 0x28, 0x30, 0x6a, 0xf0, 0x04,
	0xc1, 0xf9, 0x49, 0x6c, 0x60, 0xd7, 0x1b, 0x03, 0x02, 0x00, 0x00, 0xff, 0xff, 0x9a, 0x56, 0xdb,
	0x89, 0xcb, 0x00, 0x00, 0x00,
}
