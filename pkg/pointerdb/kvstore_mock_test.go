// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
// Code generated by MockGen. DO NOT EDIT.
// Source: storj.io/storj/storage (interfaces: KeyValueStore)

// Package pointerdb is a generated GoMock package.
package pointerdb

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	storage "storj.io/storj/storage"
)

// MockKeyValueStore is a mock of KeyValueStore interface
type MockKeyValueStore struct {
	ctrl     *gomock.Controller
	recorder *MockKeyValueStoreMockRecorder
}

// MockKeyValueStoreMockRecorder is the mock recorder for MockKeyValueStore
type MockKeyValueStoreMockRecorder struct {
	mock *MockKeyValueStore
}

// NewMockKeyValueStore creates a new mock instance
func NewMockKeyValueStore(ctrl *gomock.Controller) *MockKeyValueStore {
	mock := &MockKeyValueStore{ctrl: ctrl}
	mock.recorder = &MockKeyValueStoreMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockKeyValueStore) EXPECT() *MockKeyValueStoreMockRecorder {
	return m.recorder
}

// Close mocks base method
func (m *MockKeyValueStore) Close() error {
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close
func (mr *MockKeyValueStoreMockRecorder) Close() *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockKeyValueStore)(nil).Close))
}

// Delete mocks base method
func (m *MockKeyValueStore) Delete(arg0 storage.Key) error {
	ret := m.ctrl.Call(m, "Delete", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete
func (mr *MockKeyValueStoreMockRecorder) Delete(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockKeyValueStore)(nil).Delete), arg0)
}

// Get mocks base method
func (m *MockKeyValueStore) Get(arg0 storage.Key) (storage.Value, error) {
	ret := m.ctrl.Call(m, "Get", arg0)
	ret0, _ := ret[0].(storage.Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get
func (mr *MockKeyValueStoreMockRecorder) Get(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockKeyValueStore)(nil).Get), arg0)
}

// GetAll mocks base method
func (m *MockKeyValueStore) GetAll(arg0 storage.Keys) (storage.Values, error) {
	ret := m.ctrl.Call(m, "GetAll", arg0)
	ret0, _ := ret[0].(storage.Values)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAll indicates an expected call of GetAll
func (mr *MockKeyValueStoreMockRecorder) GetAll(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAll", reflect.TypeOf((*MockKeyValueStore)(nil).GetAll), arg0)
}

// List mocks base method
func (m *MockKeyValueStore) List(arg0 storage.ListOptions) (storage.Items, storage.More, error) {
	ret := m.ctrl.Call(m, "List", arg0)
	ret0, _ := ret[0].(storage.Items)
	ret1, _ := ret[1].(storage.More)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// List indicates an expected call of List
func (mr *MockKeyValueStoreMockRecorder) List(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockKeyValueStore)(nil).List), arg0)
}

// Put mocks base method
func (m *MockKeyValueStore) Put(arg0 storage.Key, arg1 storage.Value) error {
	ret := m.ctrl.Call(m, "Put", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Put indicates an expected call of Put
func (mr *MockKeyValueStoreMockRecorder) Put(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Put", reflect.TypeOf((*MockKeyValueStore)(nil).Put), arg0, arg1)
}

// ReverseList mocks base method
func (m *MockKeyValueStore) ReverseList(arg0 storage.Key, arg1 storage.Limit) (storage.Keys, error) {
	ret := m.ctrl.Call(m, "ReverseList", arg0, arg1)
	ret0, _ := ret[0].(storage.Keys)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ReverseList indicates an expected call of ReverseList
func (mr *MockKeyValueStoreMockRecorder) ReverseList(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ReverseList", reflect.TypeOf((*MockKeyValueStore)(nil).ReverseList), arg0, arg1)
}
