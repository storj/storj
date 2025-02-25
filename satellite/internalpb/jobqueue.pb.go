// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: jobqueue.proto

package internalpb

import (
	fmt "fmt"
	math "math"
	time "time"

	proto "github.com/gogo/protobuf/proto"

	_ "storj.io/common/pb"
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
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type RepairJob struct {
	StreamId             []byte     `protobuf:"bytes,1,opt,name=stream_id,json=streamId,proto3" json:"stream_id,omitempty"`
	Position             uint64     `protobuf:"varint,2,opt,name=position,proto3" json:"position,omitempty"`
	Priority             float64    `protobuf:"fixed64,3,opt,name=priority,proto3" json:"priority,omitempty"`
	InsertedAt           *time.Time `protobuf:"bytes,4,opt,name=inserted_at,json=insertedAt,proto3,stdtime" json:"inserted_at,omitempty"`
	LastAttemptedAt      *time.Time `protobuf:"bytes,5,opt,name=last_attempted_at,json=lastAttemptedAt,proto3,stdtime" json:"last_attempted_at,omitempty"`
	NumAttempts          int32      `protobuf:"varint,6,opt,name=num_attempts,json=numAttempts,proto3" json:"num_attempts,omitempty"`
	Placement            int32      `protobuf:"varint,7,opt,name=placement,proto3" json:"placement,omitempty"`
	NumMissing           int32      `protobuf:"varint,8,opt,name=num_missing,json=numMissing,proto3" json:"num_missing,omitempty"`
	NumOutOfPlacement    int32      `protobuf:"varint,9,opt,name=num_out_of_placement,json=numOutOfPlacement,proto3" json:"num_out_of_placement,omitempty"`
	UpdatedAt            *time.Time `protobuf:"bytes,10,opt,name=updated_at,json=updatedAt,proto3,stdtime" json:"updated_at,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *RepairJob) Reset()         { *m = RepairJob{} }
func (m *RepairJob) String() string { return proto.CompactTextString(m) }
func (*RepairJob) ProtoMessage()    {}
func (*RepairJob) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{0}
}
func (m *RepairJob) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_RepairJob.Unmarshal(m, b)
}
func (m *RepairJob) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_RepairJob.Marshal(b, m, deterministic)
}
func (m *RepairJob) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RepairJob.Merge(m, src)
}
func (m *RepairJob) XXX_Size() int {
	return xxx_messageInfo_RepairJob.Size(m)
}
func (m *RepairJob) XXX_DiscardUnknown() {
	xxx_messageInfo_RepairJob.DiscardUnknown(m)
}

var xxx_messageInfo_RepairJob proto.InternalMessageInfo

func (m *RepairJob) GetStreamId() []byte {
	if m != nil {
		return m.StreamId
	}
	return nil
}

func (m *RepairJob) GetPosition() uint64 {
	if m != nil {
		return m.Position
	}
	return 0
}

func (m *RepairJob) GetPriority() float64 {
	if m != nil {
		return m.Priority
	}
	return 0
}

func (m *RepairJob) GetInsertedAt() *time.Time {
	if m != nil {
		return m.InsertedAt
	}
	return nil
}

func (m *RepairJob) GetLastAttemptedAt() *time.Time {
	if m != nil {
		return m.LastAttemptedAt
	}
	return nil
}

func (m *RepairJob) GetNumAttempts() int32 {
	if m != nil {
		return m.NumAttempts
	}
	return 0
}

func (m *RepairJob) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

func (m *RepairJob) GetNumMissing() int32 {
	if m != nil {
		return m.NumMissing
	}
	return 0
}

func (m *RepairJob) GetNumOutOfPlacement() int32 {
	if m != nil {
		return m.NumOutOfPlacement
	}
	return 0
}

func (m *RepairJob) GetUpdatedAt() *time.Time {
	if m != nil {
		return m.UpdatedAt
	}
	return nil
}

type JobQueuePushRequest struct {
	Job                  *RepairJob `protobuf:"bytes,1,opt,name=job,proto3" json:"job,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *JobQueuePushRequest) Reset()         { *m = JobQueuePushRequest{} }
func (m *JobQueuePushRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueuePushRequest) ProtoMessage()    {}
func (*JobQueuePushRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{1}
}
func (m *JobQueuePushRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePushRequest.Unmarshal(m, b)
}
func (m *JobQueuePushRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePushRequest.Marshal(b, m, deterministic)
}
func (m *JobQueuePushRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePushRequest.Merge(m, src)
}
func (m *JobQueuePushRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueuePushRequest.Size(m)
}
func (m *JobQueuePushRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePushRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePushRequest proto.InternalMessageInfo

func (m *JobQueuePushRequest) GetJob() *RepairJob {
	if m != nil {
		return m.Job
	}
	return nil
}

type JobQueuePushResponse struct {
	NewlyInserted        bool     `protobuf:"varint,1,opt,name=newly_inserted,json=newlyInserted,proto3" json:"newly_inserted,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueuePushResponse) Reset()         { *m = JobQueuePushResponse{} }
func (m *JobQueuePushResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueuePushResponse) ProtoMessage()    {}
func (*JobQueuePushResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{2}
}
func (m *JobQueuePushResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePushResponse.Unmarshal(m, b)
}
func (m *JobQueuePushResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePushResponse.Marshal(b, m, deterministic)
}
func (m *JobQueuePushResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePushResponse.Merge(m, src)
}
func (m *JobQueuePushResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueuePushResponse.Size(m)
}
func (m *JobQueuePushResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePushResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePushResponse proto.InternalMessageInfo

func (m *JobQueuePushResponse) GetNewlyInserted() bool {
	if m != nil {
		return m.NewlyInserted
	}
	return false
}

type JobQueuePushBatchRequest struct {
	Jobs                 []*RepairJob `protobuf:"bytes,1,rep,name=jobs,proto3" json:"jobs,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *JobQueuePushBatchRequest) Reset()         { *m = JobQueuePushBatchRequest{} }
func (m *JobQueuePushBatchRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueuePushBatchRequest) ProtoMessage()    {}
func (*JobQueuePushBatchRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{3}
}
func (m *JobQueuePushBatchRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePushBatchRequest.Unmarshal(m, b)
}
func (m *JobQueuePushBatchRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePushBatchRequest.Marshal(b, m, deterministic)
}
func (m *JobQueuePushBatchRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePushBatchRequest.Merge(m, src)
}
func (m *JobQueuePushBatchRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueuePushBatchRequest.Size(m)
}
func (m *JobQueuePushBatchRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePushBatchRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePushBatchRequest proto.InternalMessageInfo

func (m *JobQueuePushBatchRequest) GetJobs() []*RepairJob {
	if m != nil {
		return m.Jobs
	}
	return nil
}

type JobQueuePushBatchResponse struct {
	NewlyInserted        []bool   `protobuf:"varint,1,rep,packed,name=newly_inserted,json=newlyInserted,proto3" json:"newly_inserted,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueuePushBatchResponse) Reset()         { *m = JobQueuePushBatchResponse{} }
func (m *JobQueuePushBatchResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueuePushBatchResponse) ProtoMessage()    {}
func (*JobQueuePushBatchResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{4}
}
func (m *JobQueuePushBatchResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePushBatchResponse.Unmarshal(m, b)
}
func (m *JobQueuePushBatchResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePushBatchResponse.Marshal(b, m, deterministic)
}
func (m *JobQueuePushBatchResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePushBatchResponse.Merge(m, src)
}
func (m *JobQueuePushBatchResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueuePushBatchResponse.Size(m)
}
func (m *JobQueuePushBatchResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePushBatchResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePushBatchResponse proto.InternalMessageInfo

func (m *JobQueuePushBatchResponse) GetNewlyInserted() []bool {
	if m != nil {
		return m.NewlyInserted
	}
	return nil
}

type JobQueuePopRequest struct {
	IncludedPlacements   []int32  `protobuf:"varint,1,rep,packed,name=included_placements,json=includedPlacements,proto3" json:"included_placements,omitempty"`
	ExcludedPlacements   []int32  `protobuf:"varint,2,rep,packed,name=excluded_placements,json=excludedPlacements,proto3" json:"excluded_placements,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueuePopRequest) Reset()         { *m = JobQueuePopRequest{} }
func (m *JobQueuePopRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueuePopRequest) ProtoMessage()    {}
func (*JobQueuePopRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{5}
}
func (m *JobQueuePopRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePopRequest.Unmarshal(m, b)
}
func (m *JobQueuePopRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePopRequest.Marshal(b, m, deterministic)
}
func (m *JobQueuePopRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePopRequest.Merge(m, src)
}
func (m *JobQueuePopRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueuePopRequest.Size(m)
}
func (m *JobQueuePopRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePopRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePopRequest proto.InternalMessageInfo

func (m *JobQueuePopRequest) GetIncludedPlacements() []int32 {
	if m != nil {
		return m.IncludedPlacements
	}
	return nil
}

func (m *JobQueuePopRequest) GetExcludedPlacements() []int32 {
	if m != nil {
		return m.ExcludedPlacements
	}
	return nil
}

type JobQueuePopResponse struct {
	Job                  *RepairJob `protobuf:"bytes,1,opt,name=job,proto3" json:"job,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *JobQueuePopResponse) Reset()         { *m = JobQueuePopResponse{} }
func (m *JobQueuePopResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueuePopResponse) ProtoMessage()    {}
func (*JobQueuePopResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{6}
}
func (m *JobQueuePopResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePopResponse.Unmarshal(m, b)
}
func (m *JobQueuePopResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePopResponse.Marshal(b, m, deterministic)
}
func (m *JobQueuePopResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePopResponse.Merge(m, src)
}
func (m *JobQueuePopResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueuePopResponse.Size(m)
}
func (m *JobQueuePopResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePopResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePopResponse proto.InternalMessageInfo

func (m *JobQueuePopResponse) GetJob() *RepairJob {
	if m != nil {
		return m.Job
	}
	return nil
}

type JobQueuePeekRequest struct {
	Placement            int32    `protobuf:"varint,1,opt,name=placement,proto3" json:"placement,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueuePeekRequest) Reset()         { *m = JobQueuePeekRequest{} }
func (m *JobQueuePeekRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueuePeekRequest) ProtoMessage()    {}
func (*JobQueuePeekRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{7}
}
func (m *JobQueuePeekRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePeekRequest.Unmarshal(m, b)
}
func (m *JobQueuePeekRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePeekRequest.Marshal(b, m, deterministic)
}
func (m *JobQueuePeekRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePeekRequest.Merge(m, src)
}
func (m *JobQueuePeekRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueuePeekRequest.Size(m)
}
func (m *JobQueuePeekRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePeekRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePeekRequest proto.InternalMessageInfo

func (m *JobQueuePeekRequest) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

type JobQueuePeekResponse struct {
	Job                  *RepairJob `protobuf:"bytes,1,opt,name=job,proto3" json:"job,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *JobQueuePeekResponse) Reset()         { *m = JobQueuePeekResponse{} }
func (m *JobQueuePeekResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueuePeekResponse) ProtoMessage()    {}
func (*JobQueuePeekResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{8}
}
func (m *JobQueuePeekResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueuePeekResponse.Unmarshal(m, b)
}
func (m *JobQueuePeekResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueuePeekResponse.Marshal(b, m, deterministic)
}
func (m *JobQueuePeekResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueuePeekResponse.Merge(m, src)
}
func (m *JobQueuePeekResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueuePeekResponse.Size(m)
}
func (m *JobQueuePeekResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueuePeekResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueuePeekResponse proto.InternalMessageInfo

func (m *JobQueuePeekResponse) GetJob() *RepairJob {
	if m != nil {
		return m.Job
	}
	return nil
}

type JobQueueLengthRequest struct {
	Placement            int32    `protobuf:"varint,1,opt,name=placement,proto3" json:"placement,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueLengthRequest) Reset()         { *m = JobQueueLengthRequest{} }
func (m *JobQueueLengthRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueueLengthRequest) ProtoMessage()    {}
func (*JobQueueLengthRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{9}
}
func (m *JobQueueLengthRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueLengthRequest.Unmarshal(m, b)
}
func (m *JobQueueLengthRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueLengthRequest.Marshal(b, m, deterministic)
}
func (m *JobQueueLengthRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueLengthRequest.Merge(m, src)
}
func (m *JobQueueLengthRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueueLengthRequest.Size(m)
}
func (m *JobQueueLengthRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueLengthRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueLengthRequest proto.InternalMessageInfo

func (m *JobQueueLengthRequest) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

type JobQueueLengthResponse struct {
	RepairLength         int64    `protobuf:"varint,1,opt,name=repair_length,json=repairLength,proto3" json:"repair_length,omitempty"`
	RetryLength          int64    `protobuf:"varint,2,opt,name=retry_length,json=retryLength,proto3" json:"retry_length,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueLengthResponse) Reset()         { *m = JobQueueLengthResponse{} }
func (m *JobQueueLengthResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueueLengthResponse) ProtoMessage()    {}
func (*JobQueueLengthResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{10}
}
func (m *JobQueueLengthResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueLengthResponse.Unmarshal(m, b)
}
func (m *JobQueueLengthResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueLengthResponse.Marshal(b, m, deterministic)
}
func (m *JobQueueLengthResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueLengthResponse.Merge(m, src)
}
func (m *JobQueueLengthResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueueLengthResponse.Size(m)
}
func (m *JobQueueLengthResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueLengthResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueLengthResponse proto.InternalMessageInfo

func (m *JobQueueLengthResponse) GetRepairLength() int64 {
	if m != nil {
		return m.RepairLength
	}
	return 0
}

func (m *JobQueueLengthResponse) GetRetryLength() int64 {
	if m != nil {
		return m.RetryLength
	}
	return 0
}

type JobQueueTruncateRequest struct {
	Placement            int32    `protobuf:"varint,1,opt,name=placement,proto3" json:"placement,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueTruncateRequest) Reset()         { *m = JobQueueTruncateRequest{} }
func (m *JobQueueTruncateRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueueTruncateRequest) ProtoMessage()    {}
func (*JobQueueTruncateRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{11}
}
func (m *JobQueueTruncateRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueTruncateRequest.Unmarshal(m, b)
}
func (m *JobQueueTruncateRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueTruncateRequest.Marshal(b, m, deterministic)
}
func (m *JobQueueTruncateRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueTruncateRequest.Merge(m, src)
}
func (m *JobQueueTruncateRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueueTruncateRequest.Size(m)
}
func (m *JobQueueTruncateRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueTruncateRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueTruncateRequest proto.InternalMessageInfo

func (m *JobQueueTruncateRequest) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

type JobQueueTruncateResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueTruncateResponse) Reset()         { *m = JobQueueTruncateResponse{} }
func (m *JobQueueTruncateResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueueTruncateResponse) ProtoMessage()    {}
func (*JobQueueTruncateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{12}
}
func (m *JobQueueTruncateResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueTruncateResponse.Unmarshal(m, b)
}
func (m *JobQueueTruncateResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueTruncateResponse.Marshal(b, m, deterministic)
}
func (m *JobQueueTruncateResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueTruncateResponse.Merge(m, src)
}
func (m *JobQueueTruncateResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueueTruncateResponse.Size(m)
}
func (m *JobQueueTruncateResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueTruncateResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueTruncateResponse proto.InternalMessageInfo

type JobQueueAddPlacementQueueRequest struct {
	Placement            int32    `protobuf:"varint,1,opt,name=placement,proto3" json:"placement,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueAddPlacementQueueRequest) Reset()         { *m = JobQueueAddPlacementQueueRequest{} }
func (m *JobQueueAddPlacementQueueRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueueAddPlacementQueueRequest) ProtoMessage()    {}
func (*JobQueueAddPlacementQueueRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{13}
}
func (m *JobQueueAddPlacementQueueRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueAddPlacementQueueRequest.Unmarshal(m, b)
}
func (m *JobQueueAddPlacementQueueRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueAddPlacementQueueRequest.Marshal(b, m, deterministic)
}
func (m *JobQueueAddPlacementQueueRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueAddPlacementQueueRequest.Merge(m, src)
}
func (m *JobQueueAddPlacementQueueRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueueAddPlacementQueueRequest.Size(m)
}
func (m *JobQueueAddPlacementQueueRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueAddPlacementQueueRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueAddPlacementQueueRequest proto.InternalMessageInfo

func (m *JobQueueAddPlacementQueueRequest) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

type JobQueueAddPlacementQueueResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueAddPlacementQueueResponse) Reset()         { *m = JobQueueAddPlacementQueueResponse{} }
func (m *JobQueueAddPlacementQueueResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueueAddPlacementQueueResponse) ProtoMessage()    {}
func (*JobQueueAddPlacementQueueResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{14}
}
func (m *JobQueueAddPlacementQueueResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueAddPlacementQueueResponse.Unmarshal(m, b)
}
func (m *JobQueueAddPlacementQueueResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueAddPlacementQueueResponse.Marshal(b, m, deterministic)
}
func (m *JobQueueAddPlacementQueueResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueAddPlacementQueueResponse.Merge(m, src)
}
func (m *JobQueueAddPlacementQueueResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueueAddPlacementQueueResponse.Size(m)
}
func (m *JobQueueAddPlacementQueueResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueAddPlacementQueueResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueAddPlacementQueueResponse proto.InternalMessageInfo

type JobQueueDestroyPlacementQueueRequest struct {
	Placement            int32    `protobuf:"varint,1,opt,name=placement,proto3" json:"placement,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueDestroyPlacementQueueRequest) Reset()         { *m = JobQueueDestroyPlacementQueueRequest{} }
func (m *JobQueueDestroyPlacementQueueRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueueDestroyPlacementQueueRequest) ProtoMessage()    {}
func (*JobQueueDestroyPlacementQueueRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{15}
}
func (m *JobQueueDestroyPlacementQueueRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueDestroyPlacementQueueRequest.Unmarshal(m, b)
}
func (m *JobQueueDestroyPlacementQueueRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueDestroyPlacementQueueRequest.Marshal(b, m, deterministic)
}
func (m *JobQueueDestroyPlacementQueueRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueDestroyPlacementQueueRequest.Merge(m, src)
}
func (m *JobQueueDestroyPlacementQueueRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueueDestroyPlacementQueueRequest.Size(m)
}
func (m *JobQueueDestroyPlacementQueueRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueDestroyPlacementQueueRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueDestroyPlacementQueueRequest proto.InternalMessageInfo

func (m *JobQueueDestroyPlacementQueueRequest) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

type JobQueueDestroyPlacementQueueResponse struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueDestroyPlacementQueueResponse) Reset()         { *m = JobQueueDestroyPlacementQueueResponse{} }
func (m *JobQueueDestroyPlacementQueueResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueueDestroyPlacementQueueResponse) ProtoMessage()    {}
func (*JobQueueDestroyPlacementQueueResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{16}
}
func (m *JobQueueDestroyPlacementQueueResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueDestroyPlacementQueueResponse.Unmarshal(m, b)
}
func (m *JobQueueDestroyPlacementQueueResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueDestroyPlacementQueueResponse.Marshal(b, m, deterministic)
}
func (m *JobQueueDestroyPlacementQueueResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueDestroyPlacementQueueResponse.Merge(m, src)
}
func (m *JobQueueDestroyPlacementQueueResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueueDestroyPlacementQueueResponse.Size(m)
}
func (m *JobQueueDestroyPlacementQueueResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueDestroyPlacementQueueResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueDestroyPlacementQueueResponse proto.InternalMessageInfo

type JobQueueInspectRequest struct {
	StreamId             []byte   `protobuf:"bytes,1,opt,name=stream_id,json=streamId,proto3" json:"stream_id,omitempty"`
	Position             uint64   `protobuf:"varint,2,opt,name=position,proto3" json:"position,omitempty"`
	Placement            int32    `protobuf:"varint,3,opt,name=placement,proto3" json:"placement,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *JobQueueInspectRequest) Reset()         { *m = JobQueueInspectRequest{} }
func (m *JobQueueInspectRequest) String() string { return proto.CompactTextString(m) }
func (*JobQueueInspectRequest) ProtoMessage()    {}
func (*JobQueueInspectRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{17}
}
func (m *JobQueueInspectRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueInspectRequest.Unmarshal(m, b)
}
func (m *JobQueueInspectRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueInspectRequest.Marshal(b, m, deterministic)
}
func (m *JobQueueInspectRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueInspectRequest.Merge(m, src)
}
func (m *JobQueueInspectRequest) XXX_Size() int {
	return xxx_messageInfo_JobQueueInspectRequest.Size(m)
}
func (m *JobQueueInspectRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueInspectRequest.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueInspectRequest proto.InternalMessageInfo

func (m *JobQueueInspectRequest) GetStreamId() []byte {
	if m != nil {
		return m.StreamId
	}
	return nil
}

func (m *JobQueueInspectRequest) GetPosition() uint64 {
	if m != nil {
		return m.Position
	}
	return 0
}

func (m *JobQueueInspectRequest) GetPlacement() int32 {
	if m != nil {
		return m.Placement
	}
	return 0
}

type JobQueueInspectResponse struct {
	Job                  *RepairJob `protobuf:"bytes,1,opt,name=job,proto3" json:"job,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *JobQueueInspectResponse) Reset()         { *m = JobQueueInspectResponse{} }
func (m *JobQueueInspectResponse) String() string { return proto.CompactTextString(m) }
func (*JobQueueInspectResponse) ProtoMessage()    {}
func (*JobQueueInspectResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_91545a11ba4fffbe, []int{18}
}
func (m *JobQueueInspectResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_JobQueueInspectResponse.Unmarshal(m, b)
}
func (m *JobQueueInspectResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_JobQueueInspectResponse.Marshal(b, m, deterministic)
}
func (m *JobQueueInspectResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_JobQueueInspectResponse.Merge(m, src)
}
func (m *JobQueueInspectResponse) XXX_Size() int {
	return xxx_messageInfo_JobQueueInspectResponse.Size(m)
}
func (m *JobQueueInspectResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_JobQueueInspectResponse.DiscardUnknown(m)
}

var xxx_messageInfo_JobQueueInspectResponse proto.InternalMessageInfo

func (m *JobQueueInspectResponse) GetJob() *RepairJob {
	if m != nil {
		return m.Job
	}
	return nil
}

func init() {
	proto.RegisterType((*RepairJob)(nil), "jobqueue.RepairJob")
	proto.RegisterType((*JobQueuePushRequest)(nil), "jobqueue.JobQueuePushRequest")
	proto.RegisterType((*JobQueuePushResponse)(nil), "jobqueue.JobQueuePushResponse")
	proto.RegisterType((*JobQueuePushBatchRequest)(nil), "jobqueue.JobQueuePushBatchRequest")
	proto.RegisterType((*JobQueuePushBatchResponse)(nil), "jobqueue.JobQueuePushBatchResponse")
	proto.RegisterType((*JobQueuePopRequest)(nil), "jobqueue.JobQueuePopRequest")
	proto.RegisterType((*JobQueuePopResponse)(nil), "jobqueue.JobQueuePopResponse")
	proto.RegisterType((*JobQueuePeekRequest)(nil), "jobqueue.JobQueuePeekRequest")
	proto.RegisterType((*JobQueuePeekResponse)(nil), "jobqueue.JobQueuePeekResponse")
	proto.RegisterType((*JobQueueLengthRequest)(nil), "jobqueue.JobQueueLengthRequest")
	proto.RegisterType((*JobQueueLengthResponse)(nil), "jobqueue.JobQueueLengthResponse")
	proto.RegisterType((*JobQueueTruncateRequest)(nil), "jobqueue.JobQueueTruncateRequest")
	proto.RegisterType((*JobQueueTruncateResponse)(nil), "jobqueue.JobQueueTruncateResponse")
	proto.RegisterType((*JobQueueAddPlacementQueueRequest)(nil), "jobqueue.JobQueueAddPlacementQueueRequest")
	proto.RegisterType((*JobQueueAddPlacementQueueResponse)(nil), "jobqueue.JobQueueAddPlacementQueueResponse")
	proto.RegisterType((*JobQueueDestroyPlacementQueueRequest)(nil), "jobqueue.JobQueueDestroyPlacementQueueRequest")
	proto.RegisterType((*JobQueueDestroyPlacementQueueResponse)(nil), "jobqueue.JobQueueDestroyPlacementQueueResponse")
	proto.RegisterType((*JobQueueInspectRequest)(nil), "jobqueue.JobQueueInspectRequest")
	proto.RegisterType((*JobQueueInspectResponse)(nil), "jobqueue.JobQueueInspectResponse")
}

func init() { proto.RegisterFile("jobqueue.proto", fileDescriptor_91545a11ba4fffbe) }

var fileDescriptor_91545a11ba4fffbe = []byte{
	// 829 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x56, 0xdd, 0x8e, 0xdb, 0x44,
	0x14, 0x96, 0xd7, 0xbb, 0x5b, 0xe7, 0x64, 0x5b, 0xb4, 0xb3, 0x2d, 0x18, 0xd3, 0x12, 0xaf, 0x43,
	0xd4, 0x08, 0xa4, 0x58, 0xda, 0x0a, 0x71, 0x03, 0x52, 0x93, 0xb6, 0x12, 0x5b, 0x15, 0x9a, 0x5a,
	0x7b, 0xc5, 0x8d, 0xb1, 0x93, 0xd9, 0xd4, 0xa9, 0x3d, 0xe3, 0x7a, 0xc6, 0xd0, 0x3c, 0x01, 0xb7,
	0x3c, 0x16, 0xb7, 0xbc, 0x00, 0xbc, 0x0a, 0xf2, 0x78, 0xc6, 0xce, 0x8f, 0x13, 0x92, 0xde, 0x65,
	0xce, 0xf9, 0xbe, 0xef, 0x7c, 0xe3, 0x73, 0xce, 0x28, 0x70, 0x6f, 0x4e, 0xc3, 0xf7, 0x39, 0xce,
	0xf1, 0x20, 0xcd, 0x28, 0xa7, 0xc8, 0x50, 0x67, 0x0b, 0x66, 0x74, 0x46, 0xcb, 0xa8, 0xd5, 0x99,
	0x51, 0x3a, 0x8b, 0xb1, 0x2b, 0x4e, 0x61, 0x7e, 0xeb, 0xf2, 0x28, 0xc1, 0x8c, 0x07, 0x49, 0x5a,
	0x02, 0x9c, 0xbf, 0x75, 0x68, 0x79, 0x38, 0x0d, 0xa2, 0xec, 0x25, 0x0d, 0xd1, 0x17, 0xd0, 0x62,
	0x3c, 0xc3, 0x41, 0xe2, 0x47, 0x53, 0x53, 0xb3, 0xb5, 0xfe, 0x99, 0x67, 0x94, 0x81, 0xeb, 0x29,
	0xb2, 0xc0, 0x48, 0x29, 0x8b, 0x78, 0x44, 0x89, 0x79, 0x64, 0x6b, 0xfd, 0x63, 0xaf, 0x3a, 0x8b,
	0x5c, 0x16, 0xd1, 0x2c, 0xe2, 0x0b, 0x53, 0xb7, 0xb5, 0xbe, 0xe6, 0x55, 0x67, 0xf4, 0x02, 0xda,
	0x11, 0x61, 0x38, 0xe3, 0x78, 0xea, 0x07, 0xdc, 0x3c, 0xb6, 0xb5, 0x7e, 0xfb, 0xca, 0x1a, 0x94,
	0xce, 0x06, 0xca, 0xd9, 0xe0, 0x46, 0x39, 0x1b, 0x19, 0x7f, 0xfd, 0xd3, 0xd1, 0xfe, 0xfc, 0xb7,
	0xa3, 0x79, 0xa0, 0x88, 0x43, 0x8e, 0xc6, 0x70, 0x1e, 0x07, 0x8c, 0xfb, 0x01, 0xe7, 0x38, 0x49,
	0xa5, 0xd8, 0xc9, 0x01, 0x62, 0x9f, 0x14, 0xf4, 0xa1, 0x62, 0x0f, 0x39, 0xba, 0x84, 0x33, 0x92,
	0x27, 0x4a, 0x90, 0x99, 0xa7, 0xb6, 0xd6, 0x3f, 0xf1, 0xda, 0x24, 0x4f, 0x24, 0x8a, 0xa1, 0x87,
	0xd0, 0x4a, 0xe3, 0x60, 0x82, 0x13, 0x4c, 0xb8, 0x79, 0x47, 0xe4, 0xeb, 0x00, 0xea, 0x40, 0x01,
	0xf6, 0x93, 0x88, 0xb1, 0x88, 0xcc, 0x4c, 0x43, 0xe4, 0x81, 0xe4, 0xc9, 0x4f, 0x65, 0x04, 0xb9,
	0x70, 0xbf, 0x00, 0xd0, 0x9c, 0xfb, 0xf4, 0xd6, 0xaf, 0x95, 0x5a, 0x02, 0x79, 0x4e, 0xf2, 0xe4,
	0x75, 0xce, 0x5f, 0xdf, 0x8e, 0x2b, 0xc5, 0x67, 0x00, 0x79, 0x3a, 0x0d, 0xe4, 0xed, 0xe0, 0x80,
	0xdb, 0xb5, 0x24, 0x6f, 0xc8, 0x9d, 0xef, 0xe1, 0xe2, 0x25, 0x0d, 0xdf, 0x14, 0xc3, 0x30, 0xce,
	0xd9, 0x5b, 0x0f, 0xbf, 0xcf, 0x31, 0xe3, 0xa8, 0x07, 0xfa, 0x9c, 0x86, 0xa2, 0xad, 0xed, 0xab,
	0x8b, 0x41, 0x35, 0x3f, 0x55, 0xfb, 0xbd, 0x22, 0xef, 0xfc, 0x00, 0xf7, 0x57, 0xd9, 0x2c, 0xa5,
	0x84, 0x61, 0xd4, 0x83, 0x7b, 0x04, 0xff, 0x1e, 0x2f, 0x7c, 0xd5, 0x13, 0xa1, 0x64, 0x78, 0x77,
	0x45, 0xf4, 0x5a, 0x06, 0x9d, 0x67, 0x60, 0x2e, 0xd3, 0x47, 0x01, 0x9f, 0x54, 0x0e, 0x1e, 0xc3,
	0xf1, 0x9c, 0x86, 0xcc, 0xd4, 0x6c, 0x7d, 0x9b, 0x05, 0x01, 0x70, 0x46, 0xf0, 0x79, 0x83, 0xc8,
	0x0e, 0x23, 0xfa, 0xa6, 0x91, 0xdf, 0x00, 0x55, 0x1a, 0x34, 0x55, 0x16, 0x5c, 0xb8, 0x88, 0xc8,
	0x24, 0xce, 0xa7, 0x78, 0x5a, 0xf7, 0xa3, 0x74, 0x74, 0xe2, 0x21, 0x95, 0xaa, 0x1a, 0xc2, 0x0a,
	0x02, 0xfe, 0xb0, 0x49, 0x38, 0x2a, 0x09, 0x2a, 0x55, 0x13, 0x56, 0xbe, 0x7e, 0x51, 0xb7, 0x72,
	0xbd, 0xd7, 0xd7, 0x7f, 0xb2, 0xc4, 0xc6, 0xf8, 0x9d, 0xb2, 0xbd, 0x32, 0x87, 0xda, 0xda, 0x1c,
	0xae, 0xb4, 0x4c, 0x90, 0x0e, 0xab, 0xf9, 0x2d, 0x3c, 0x50, 0xf4, 0x57, 0x98, 0xcc, 0xf8, 0xdb,
	0xfd, 0xaa, 0xfe, 0x0a, 0x9f, 0xae, 0xd3, 0x64, 0xdd, 0x2e, 0xdc, 0xcd, 0x44, 0x09, 0x3f, 0x16,
	0x09, 0xc1, 0xd5, 0xbd, 0xb3, 0x32, 0x58, 0x82, 0x8b, 0xed, 0xcb, 0x30, 0xcf, 0x16, 0x0a, 0x73,
	0x24, 0x30, 0x6d, 0x11, 0x2b, 0x21, 0xce, 0x77, 0xf0, 0x99, 0xaa, 0x70, 0x93, 0xe5, 0x64, 0x12,
	0x70, 0xbc, 0x9f, 0x35, 0xab, 0x1e, 0xc2, 0x9a, 0x58, 0x9a, 0x73, 0x9e, 0x82, 0xad, 0x72, 0xc3,
	0x69, 0xdd, 0x38, 0x11, 0xd8, 0x4f, 0xbd, 0x0b, 0x97, 0x3b, 0x14, 0x64, 0x99, 0xe7, 0xf0, 0x95,
	0x02, 0x3d, 0xc7, 0x8c, 0x67, 0x74, 0xf1, 0x31, 0xa5, 0x1e, 0x43, 0xef, 0x7f, 0x54, 0x64, 0x39,
	0x5a, 0x37, 0xe3, 0x9a, 0xb0, 0x14, 0x4f, 0xb8, 0x2a, 0xf0, 0xd1, 0x6f, 0xfa, 0x8a, 0x33, 0x7d,
	0xdd, 0xd9, 0xd3, 0xba, 0x37, 0x55, 0xc1, 0x83, 0xc6, 0xee, 0xea, 0x8f, 0x53, 0x30, 0x94, 0x04,
	0x7a, 0x01, 0xc7, 0xc5, 0xa6, 0xa3, 0x47, 0x35, 0xbc, 0xe1, 0x0d, 0xb3, 0xbe, 0xdc, 0x96, 0x96,
	0xa5, 0x6f, 0xa0, 0x55, 0x3d, 0x18, 0xc8, 0x69, 0x06, 0x2f, 0x3f, 0x49, 0x56, 0x77, 0x27, 0x46,
	0xaa, 0x8e, 0x40, 0x1f, 0xd3, 0x14, 0x3d, 0x6c, 0xc0, 0x56, 0x2f, 0x8b, 0xf5, 0x68, 0x4b, 0x56,
	0x6a, 0x14, 0x17, 0xc4, 0xf8, 0x5d, 0xe3, 0x05, 0xeb, 0x45, 0x6f, 0xbc, 0xe0, 0xf2, 0x4a, 0xff,
	0x08, 0xfa, 0x2b, 0x4c, 0x50, 0x67, 0x13, 0xb6, 0xb2, 0xba, 0x96, 0xbd, 0x1d, 0x20, 0x95, 0x7e,
	0x86, 0x3b, 0xb2, 0x71, 0xa8, 0x01, 0xbc, 0x3a, 0x44, 0xd6, 0xe5, 0x0e, 0x84, 0xd4, 0x7b, 0x03,
	0x86, 0xda, 0x35, 0xd4, 0x00, 0x5f, 0x5b, 0x60, 0xcb, 0xd9, 0x05, 0x91, 0x92, 0x31, 0x9c, 0x6f,
	0x2c, 0x18, 0xfa, 0x7a, 0x93, 0xb8, 0x6d, 0x8f, 0xad, 0x6f, 0xf6, 0xc2, 0xca, 0x6a, 0x1f, 0xe0,
	0x41, 0xe3, 0x8e, 0xa1, 0xc1, 0xa6, 0xca, 0xae, 0x95, 0xb6, 0xdc, 0xbd, 0xf1, 0x65, 0xe5, 0x51,
	0xef, 0x97, 0x2e, 0xe3, 0x34, 0x9b, 0x0f, 0x22, 0xea, 0x8a, 0x1f, 0x2e, 0x0b, 0x38, 0x8e, 0xe3,
	0x88, 0x63, 0x37, 0x22, 0x1c, 0x67, 0x24, 0x88, 0xd3, 0x30, 0x3c, 0x15, 0x7f, 0x00, 0x9e, 0xfc,
	0x17, 0x00, 0x00, 0xff, 0xff, 0xc1, 0xbb, 0xde, 0x56, 0xfb, 0x09, 0x00, 0x00,
}
