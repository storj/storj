// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// Code generated by MockGen. DO NOT EDIT.
// Source: storj.io/storj/pkg/pb (interfaces: PieceStoreRoutesClient,PieceStoreRoutes_RetrieveClient)

// Package pb is a generated GoMock package.
package pb

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	grpc "google.golang.org/grpc"
	metadata "google.golang.org/grpc/metadata"
)

// MockPieceStoreRoutesClient is a mock of PieceStoreRoutesClient interface
type MockPieceStoreRoutesClient struct {
	ctrl     *gomock.Controller
	recorder *MockPieceStoreRoutesClientMockRecorder
}

// MockPieceStoreRoutesClientMockRecorder is the mock recorder for MockPieceStoreRoutesClient
type MockPieceStoreRoutesClientMockRecorder struct {
	mock *MockPieceStoreRoutesClient
}

// NewMockPieceStoreRoutesClient creates a new mock instance
func NewMockPieceStoreRoutesClient(ctrl *gomock.Controller) *MockPieceStoreRoutesClient {
	mock := &MockPieceStoreRoutesClient{ctrl: ctrl}
	mock.recorder = &MockPieceStoreRoutesClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPieceStoreRoutesClient) EXPECT() *MockPieceStoreRoutesClientMockRecorder {
	return m.recorder
}

// Dashboard returns an object that mocks out the dashboard calls to pass tests
func (m *MockPieceStoreRoutesClient) Dashboard(ctx context.Context, req *DashboardReq, opts ...grpc.CallOption) (PieceStoreRoutes_DashboardClient, error) {
	return nil, nil
}

// Delete mocks base method
func (m *MockPieceStoreRoutesClient) Delete(arg0 context.Context, arg1 *PieceDelete, arg2 ...grpc.CallOption) (*PieceDeleteSummary, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Delete", varargs...)
	ret0, _ := ret[0].(*PieceDeleteSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Delete indicates an expected call of Delete
func (mr *MockPieceStoreRoutesClientMockRecorder) Delete(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockPieceStoreRoutesClient)(nil).Delete), varargs...)
}

// Piece mocks base method
func (m *MockPieceStoreRoutesClient) Piece(arg0 context.Context, arg1 *PieceId, arg2 ...grpc.CallOption) (*PieceSummary, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Piece", varargs...)
	ret0, _ := ret[0].(*PieceSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Piece indicates an expected call of Piece
func (mr *MockPieceStoreRoutesClientMockRecorder) Piece(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Piece", reflect.TypeOf((*MockPieceStoreRoutesClient)(nil).Piece), varargs...)
}

// Retrieve mocks base method
func (m *MockPieceStoreRoutesClient) Retrieve(arg0 context.Context, arg1 ...grpc.CallOption) (PieceStoreRoutes_RetrieveClient, error) {
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Retrieve", varargs...)
	ret0, _ := ret[0].(PieceStoreRoutes_RetrieveClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Retrieve indicates an expected call of Retrieve
func (mr *MockPieceStoreRoutesClientMockRecorder) Retrieve(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Retrieve", reflect.TypeOf((*MockPieceStoreRoutesClient)(nil).Retrieve), varargs...)
}

// Stats mocks base method
func (m *MockPieceStoreRoutesClient) Stats(arg0 context.Context, arg1 *StatsReq, arg2 ...grpc.CallOption) (*StatSummary, error) {
	varargs := []interface{}{arg0, arg1}
	for _, a := range arg2 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Stats", varargs...)
	ret0, _ := ret[0].(*StatSummary)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Stats indicates an expected call of Stats
func (mr *MockPieceStoreRoutesClientMockRecorder) Stats(arg0, arg1 interface{}, arg2 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0, arg1}, arg2...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stats", reflect.TypeOf((*MockPieceStoreRoutesClient)(nil).Stats), varargs...)
}

// Store mocks base method
func (m *MockPieceStoreRoutesClient) Store(arg0 context.Context, arg1 ...grpc.CallOption) (PieceStoreRoutes_StoreClient, error) {
	varargs := []interface{}{arg0}
	for _, a := range arg1 {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "Store", varargs...)
	ret0, _ := ret[0].(PieceStoreRoutes_StoreClient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Store indicates an expected call of Store
func (mr *MockPieceStoreRoutesClientMockRecorder) Store(arg0 interface{}, arg1 ...interface{}) *gomock.Call {
	varargs := append([]interface{}{arg0}, arg1...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Store", reflect.TypeOf((*MockPieceStoreRoutesClient)(nil).Store), varargs...)
}

// MockPieceStoreRoutes_RetrieveClient is a mock of PieceStoreRoutes_RetrieveClient interface
type MockPieceStoreRoutes_RetrieveClient struct {
	ctrl     *gomock.Controller
	recorder *MockPieceStoreRoutes_RetrieveClientMockRecorder
}

// MockPieceStoreRoutes_RetrieveClientMockRecorder is the mock recorder for MockPieceStoreRoutes_RetrieveClient
type MockPieceStoreRoutes_RetrieveClientMockRecorder struct {
	mock *MockPieceStoreRoutes_RetrieveClient
}

// NewMockPieceStoreRoutes_RetrieveClient creates a new mock instance
func NewMockPieceStoreRoutes_RetrieveClient(ctrl *gomock.Controller) *MockPieceStoreRoutes_RetrieveClient {
	mock := &MockPieceStoreRoutes_RetrieveClient{ctrl: ctrl}
	mock.recorder = &MockPieceStoreRoutes_RetrieveClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPieceStoreRoutes_RetrieveClient) EXPECT() *MockPieceStoreRoutes_RetrieveClientMockRecorder {
	return m.recorder
}

// CloseSend mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) CloseSend() error {
	ret := m.ctrl.Call(m, "CloseSend")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseSend indicates an expected call of CloseSend
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) CloseSend() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseSend", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).CloseSend))
}

// Context mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) Context() context.Context {
	ret := m.ctrl.Call(m, "Context")
	ret0, _ := ret[0].(context.Context)
	return ret0
}

// Context indicates an expected call of Context
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) Context() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Context", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).Context))
}

// Header mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) Header() (metadata.MD, error) {
	ret := m.ctrl.Call(m, "Header")
	ret0, _ := ret[0].(metadata.MD)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Header indicates an expected call of Header
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) Header() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Header", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).Header))
}

// Recv mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) Recv() (*PieceRetrievalStream, error) {
	ret := m.ctrl.Call(m, "Recv")
	ret0, _ := ret[0].(*PieceRetrievalStream)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Recv indicates an expected call of Recv
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) Recv() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Recv", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).Recv))
}

// RecvMsg mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) RecvMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "RecvMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// RecvMsg indicates an expected call of RecvMsg
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) RecvMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RecvMsg", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).RecvMsg), arg0)
}

// Send mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) Send(arg0 *PieceRetrieval) error {
	ret := m.ctrl.Call(m, "Send", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Send indicates an expected call of Send
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) Send(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Send", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).Send), arg0)
}

// SendMsg mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) SendMsg(arg0 interface{}) error {
	ret := m.ctrl.Call(m, "SendMsg", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// SendMsg indicates an expected call of SendMsg
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) SendMsg(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendMsg", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).SendMsg), arg0)
}

// Trailer mocks base method
func (m *MockPieceStoreRoutes_RetrieveClient) Trailer() metadata.MD {
	ret := m.ctrl.Call(m, "Trailer")
	ret0, _ := ret[0].(metadata.MD)
	return ret0
}

// Trailer indicates an expected call of Trailer
func (mr *MockPieceStoreRoutes_RetrieveClientMockRecorder) Trailer() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Trailer", reflect.TypeOf((*MockPieceStoreRoutes_RetrieveClient)(nil).Trailer))
}
