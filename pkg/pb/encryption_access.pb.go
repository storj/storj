// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: encryption_access.proto

package pb

import (
	fmt "fmt"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
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

type EncryptionAccess struct {
	DefaultKey           []byte                         `protobuf:"bytes,1,opt,name=default_key,json=defaultKey,proto3" json:"default_key,omitempty"`
	StoreEntries         []*EncryptionAccess_StoreEntry `protobuf:"bytes,2,rep,name=store_entries,json=storeEntries,proto3" json:"store_entries,omitempty"`
	XXX_NoUnkeyedLiteral struct{}                       `json:"-"`
	XXX_unrecognized     []byte                         `json:"-"`
	XXX_sizecache        int32                          `json:"-"`
}

func (m *EncryptionAccess) Reset()         { *m = EncryptionAccess{} }
func (m *EncryptionAccess) String() string { return proto.CompactTextString(m) }
func (*EncryptionAccess) ProtoMessage()    {}
func (*EncryptionAccess) Descriptor() ([]byte, []int) {
	return fileDescriptor_464b1a18bff4a17b, []int{0}
}
func (m *EncryptionAccess) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EncryptionAccess.Unmarshal(m, b)
}
func (m *EncryptionAccess) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EncryptionAccess.Marshal(b, m, deterministic)
}
func (m *EncryptionAccess) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EncryptionAccess.Merge(m, src)
}
func (m *EncryptionAccess) XXX_Size() int {
	return xxx_messageInfo_EncryptionAccess.Size(m)
}
func (m *EncryptionAccess) XXX_DiscardUnknown() {
	xxx_messageInfo_EncryptionAccess.DiscardUnknown(m)
}

var xxx_messageInfo_EncryptionAccess proto.InternalMessageInfo

func (m *EncryptionAccess) GetDefaultKey() []byte {
	if m != nil {
		return m.DefaultKey
	}
	return nil
}

func (m *EncryptionAccess) GetStoreEntries() []*EncryptionAccess_StoreEntry {
	if m != nil {
		return m.StoreEntries
	}
	return nil
}

type EncryptionAccess_StoreEntry struct {
	Bucket               []byte   `protobuf:"bytes,1,opt,name=bucket,proto3" json:"bucket,omitempty"`
	UnencryptedPath      []byte   `protobuf:"bytes,2,opt,name=unencrypted_path,json=unencryptedPath,proto3" json:"unencrypted_path,omitempty"`
	EncryptedPath        []byte   `protobuf:"bytes,3,opt,name=encrypted_path,json=encryptedPath,proto3" json:"encrypted_path,omitempty"`
	Key                  []byte   `protobuf:"bytes,4,opt,name=key,proto3" json:"key,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *EncryptionAccess_StoreEntry) Reset()         { *m = EncryptionAccess_StoreEntry{} }
func (m *EncryptionAccess_StoreEntry) String() string { return proto.CompactTextString(m) }
func (*EncryptionAccess_StoreEntry) ProtoMessage()    {}
func (*EncryptionAccess_StoreEntry) Descriptor() ([]byte, []int) {
	return fileDescriptor_464b1a18bff4a17b, []int{0, 0}
}
func (m *EncryptionAccess_StoreEntry) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_EncryptionAccess_StoreEntry.Unmarshal(m, b)
}
func (m *EncryptionAccess_StoreEntry) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_EncryptionAccess_StoreEntry.Marshal(b, m, deterministic)
}
func (m *EncryptionAccess_StoreEntry) XXX_Merge(src proto.Message) {
	xxx_messageInfo_EncryptionAccess_StoreEntry.Merge(m, src)
}
func (m *EncryptionAccess_StoreEntry) XXX_Size() int {
	return xxx_messageInfo_EncryptionAccess_StoreEntry.Size(m)
}
func (m *EncryptionAccess_StoreEntry) XXX_DiscardUnknown() {
	xxx_messageInfo_EncryptionAccess_StoreEntry.DiscardUnknown(m)
}

var xxx_messageInfo_EncryptionAccess_StoreEntry proto.InternalMessageInfo

func (m *EncryptionAccess_StoreEntry) GetBucket() []byte {
	if m != nil {
		return m.Bucket
	}
	return nil
}

func (m *EncryptionAccess_StoreEntry) GetUnencryptedPath() []byte {
	if m != nil {
		return m.UnencryptedPath
	}
	return nil
}

func (m *EncryptionAccess_StoreEntry) GetEncryptedPath() []byte {
	if m != nil {
		return m.EncryptedPath
	}
	return nil
}

func (m *EncryptionAccess_StoreEntry) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func init() {
	proto.RegisterType((*EncryptionAccess)(nil), "encryption_ctx.EncryptionAccess")
	proto.RegisterType((*EncryptionAccess_StoreEntry)(nil), "encryption_ctx.EncryptionAccess.StoreEntry")
}

func init() { proto.RegisterFile("encryption_access.proto", fileDescriptor_464b1a18bff4a17b) }

var fileDescriptor_464b1a18bff4a17b = []byte{
	// 231 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x12, 0x4f, 0xcd, 0x4b, 0x2e,
	0xaa, 0x2c, 0x28, 0xc9, 0xcc, 0xcf, 0x8b, 0x4f, 0x4c, 0x4e, 0x4e, 0x2d, 0x2e, 0xd6, 0x2b, 0x28,
	0xca, 0x2f, 0xc9, 0x17, 0xe2, 0x43, 0x92, 0x48, 0x2e, 0xa9, 0x90, 0xe2, 0x4a, 0xcf, 0x4f, 0xcf,
	0x87, 0xc8, 0x29, 0x4d, 0x60, 0xe2, 0x12, 0x70, 0x85, 0x4b, 0x3b, 0x82, 0xb5, 0x09, 0xc9, 0x73,
	0x71, 0xa7, 0xa4, 0xa6, 0x25, 0x96, 0xe6, 0x94, 0xc4, 0x67, 0xa7, 0x56, 0x4a, 0x30, 0x2a, 0x30,
	0x6a, 0xf0, 0x04, 0x71, 0x41, 0x85, 0xbc, 0x53, 0x2b, 0x85, 0x02, 0xb8, 0x78, 0x8b, 0x4b, 0xf2,
	0x8b, 0x52, 0xe3, 0x53, 0xf3, 0x4a, 0x8a, 0x32, 0x53, 0x8b, 0x25, 0x98, 0x14, 0x98, 0x35, 0xb8,
	0x8d, 0xb4, 0xf5, 0x50, 0x6d, 0xd2, 0x43, 0x37, 0x59, 0x2f, 0x18, 0xa4, 0xcb, 0x35, 0xaf, 0xa4,
	0xa8, 0x32, 0x88, 0xa7, 0x18, 0xc6, 0xce, 0x4c, 0x2d, 0x96, 0xea, 0x60, 0xe4, 0xe2, 0x42, 0x48,
	0x0a, 0x89, 0x71, 0xb1, 0x25, 0x95, 0x26, 0x67, 0xa7, 0x96, 0x40, 0x2d, 0x87, 0xf2, 0x84, 0x34,
	0xb9, 0x04, 0x4a, 0xf3, 0xa0, 0x96, 0xa4, 0xa6, 0xc4, 0x17, 0x24, 0x96, 0x64, 0x48, 0x30, 0x81,
	0x55, 0xf0, 0x23, 0x89, 0x07, 0x24, 0x96, 0x64, 0x08, 0xa9, 0x72, 0xf1, 0xa1, 0x29, 0x64, 0x06,
	0x2b, 0xe4, 0x45, 0x55, 0x26, 0xc0, 0xc5, 0x0c, 0xf2, 0x23, 0x0b, 0x58, 0x0e, 0xc4, 0x74, 0x62,
	0x89, 0x62, 0x2a, 0x48, 0x4a, 0x62, 0x03, 0x87, 0x8f, 0x31, 0x20, 0x00, 0x00, 0xff, 0xff, 0xf4,
	0x11, 0xcb, 0x84, 0x56, 0x01, 0x00, 0x00,
}
