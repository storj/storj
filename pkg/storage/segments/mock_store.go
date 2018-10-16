// Code generated by MockGen. DO NOT EDIT.
// Source: storj.io/storj/pkg/storage/segments (interfaces: Store)

// Package segments is a generated GoMock package.
package segments

import (
	context "context"
	io "io"
	reflect "reflect"
	time "time"

	gomock "github.com/golang/mock/gomock"

	paths "storj.io/storj/pkg/paths"
	ranger "storj.io/storj/pkg/ranger"
)

// MockStore is a mock of Store interface
type MockStore struct {
	ctrl     *gomock.Controller
	recorder *MockStoreMockRecorder
}

// MockStoreMockRecorder is the mock recorder for MockStore
type MockStoreMockRecorder struct {
	mock *MockStore
}

// NewMockStore creates a new mock instance
func NewMockStore(ctrl *gomock.Controller) *MockStore {
	mock := &MockStore{ctrl: ctrl}
	mock.recorder = &MockStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockStore) EXPECT() *MockStoreMockRecorder {
	return m.recorder
}

// Delete mocks base method
func (m *MockStore) Delete(arg0 context.Context, arg1 paths.Path) error {
	ret := m.ctrl.Call(m, "Delete", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockStoreMockRecorder) Delete(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockStore)(nil).Delete), arg0, arg1)
}

// Get mocks base method
func (m *MockStore) Get(arg0 context.Context, arg1 paths.Path) (ranger.Ranger, Meta, error) {
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(ranger.Ranger)
	ret1, _ := ret[1].(Meta)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Get indicates an expected call of Get
func (mr *MockStoreMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockStore)(nil).Get), arg0, arg1)
}

// List mocks base method
func (m *MockStore) List(arg0 context.Context, arg1, arg2, arg3 paths.Path, arg4 bool, arg5 int, arg6 uint32) ([]ListItem, bool, error) {
	ret := m.ctrl.Call(m, "List", arg0, arg1, arg2, arg3, arg4, arg5, arg6)
	ret0, _ := ret[0].([]ListItem)
	ret1, _ := ret[1].(bool)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// List indicates an expected call of List
func (mr *MockStoreMockRecorder) List(arg0, arg1, arg2, arg3, arg4, arg5, arg6 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockStore)(nil).List), arg0, arg1, arg2, arg3, arg4, arg5, arg6)
}

// Meta mocks base method
func (m *MockStore) Meta(arg0 context.Context, arg1 paths.Path) (Meta, error) {
	ret := m.ctrl.Call(m, "Meta", arg0, arg1)
	ret0, _ := ret[0].(Meta)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Meta indicates an expected call of Meta
func (mr *MockStoreMockRecorder) Meta(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Meta", reflect.TypeOf((*MockStore)(nil).Meta), arg0, arg1)
}

// Put mocks base method
func (m *MockStore) Put(arg0 context.Context, arg1 io.Reader, arg2 time.Time, arg3 func() (paths.Path, []byte, error)) (Meta, error) {
	ret := m.ctrl.Call(m, "Put", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(Meta)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Put indicates an expected call of Put
func (mr *MockStoreMockRecorder) Put(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockStore)(nil).Put), arg0, arg1, arg2, arg3)
}
