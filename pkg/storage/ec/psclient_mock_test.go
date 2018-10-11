// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// Code generated by MockGen. DO NOT EDIT.
// Source: storj.io/storj/pkg/piecestore/rpc/client (interfaces: PSClient)

// mockgen -destination=pkg/storage/ec/psclient_mock_test.go storj.io/storj/pkg/piecestore/rpc/client PSClient

// Package ecclient is a generated GoMock package.
package ecclient

import (
	context "context"
	io "io"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"
	pb "storj.io/storj/pkg/pb"
	client "storj.io/storj/pkg/piecestore/rpc/client"
	ranger "storj.io/storj/pkg/ranger"
)

// MockPSClient is a mock of PSClient interface
type MockPSClient struct {
	ctrl     *gomock.Controller
	recorder *MockPSClientMockRecorder
}

// MockPSClientMockRecorder is the mock recorder for MockPSClient
type MockPSClientMockRecorder struct {
	mock *MockPSClient
}

// NewMockPSClient creates a new mock instance
func NewMockPSClient(ctrl *gomock.Controller) *MockPSClient {
	mock := &MockPSClient{ctrl: ctrl}
	mock.recorder = &MockPSClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPSClient) EXPECT() *MockPSClientMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockPSClient) Close() error {
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockPSClientMockRecorder) Close() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockPSClient)(nil).Close))
}

// Delete mocks base method
func (m *MockPSClient) Delete(arg0 context.Context, arg1 client.PieceID) error {
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockPSClientMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockPSClient)(nil).Delete), arg0, arg1)
}

// Get mocks base method
func (m *MockPSClient) Get(arg0 context.Context, arg1 client.PieceID, arg2 int64, arg3 *pb.PayerBandwidthAllocation) (ranger.Ranger, error) {
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(ranger.Ranger)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockPSClientMockRecorder) Get(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockPSClient)(nil).Get), arg0, arg1, arg2, arg3)
}

// Meta mocks base method
func (m *MockPSClient) Meta(arg0 context.Context, arg1 client.PieceID) (*pb.PieceSummary, error) {
	ret := m.ctrl.Call(m, "Meta", arg0, arg1)
	ret0, _ := ret[0].(*pb.PieceSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Meta indicates an expected call of Meta
func (mr *MockPSClientMockRecorder) Meta(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Meta", reflect.TypeOf((*MockPSClient)(nil).Meta), arg0, arg1)
}

// Put mocks base method
func (m *MockPSClient) Put(arg0 context.Context, arg1 client.PieceID, arg2 io.Reader, arg3 time.Time, arg4 *pb.PayerBandwidthAllocation) error {
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2, arg3, arg4)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockPSClientMockRecorder) Put(arg0, arg1, arg2, arg3, arg4 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockPSClient)(nil).Put), arg0, arg1, arg2, arg3, arg4)
}

// Stats mocks base method
func (m *MockPSClient) Stats(arg0 context.Context) (*pb.StatSummary, error) {
	ret := m.ctrl.Call(m, "Stats", arg0)
	ret0, _ := ret[0].(*pb.StatSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stats indicates an expected call of Stats
func (mr *MockPSClientMockRecorder) Stats(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stats", reflect.TypeOf((*MockPSClient)(nil).Stats), arg0)
}
