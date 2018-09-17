// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// Code generated by MockGen. DO NOT EDIT.
// Source: storj.io/storj/pkg/transport (interfaces: Client)

// Package ecclient is a generated GoMock package.
package ecclient

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	grpc "google.golang.org/grpc"
	"storj.io/storj/pkg/pb"
)

// MockClient is a mock of Client interface
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// DialNode mocks base method
func (m *MockClient) DialNode(arg0 context.Context, arg1 *pb.Node) (*grpc.ClientConn, error) {
	ret := m.ctrl.Call(m, "DialNode", arg0, arg1)
	ret0, _ := ret[0].(*grpc.ClientConn)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DialNode indicates an expected call of DialNode
func (mr *MockClientMockRecorder) DialNode(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DialNode", reflect.TypeOf((*MockClient)(nil).DialNode), arg0, arg1)
}

// DialUnauthenticated mocks base method
func (m *MockClient) DialUnauthenticated(arg0 context.Context, arg1 pb.NodeAddress) (*grpc.ClientConn, error) {
	ret := m.ctrl.Call(m, "DialUnauthenticated", arg0, arg1)
	ret0, _ := ret[0].(*grpc.ClientConn)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DialUnauthenticated indicates an expected call of DialUnauthenticated
func (mr *MockClientMockRecorder) DialUnauthenticated(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DialUnauthenticated", reflect.TypeOf((*MockClient)(nil).DialUnauthenticated), arg0, arg1)
}
