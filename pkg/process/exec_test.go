// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process_test

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/process"
)

type MockedService struct {
	mock.Mock
}

func (m *MockedService) InstanceID() string {
	return ""
}

func (m *MockedService) Process(ctx context.Context, cmd *cobra.Command, args []string) error {
	arguments := m.Called(ctx, cmd, args)
	return arguments.Error(0)
}

func (m *MockedService) SetLogger(*zap.Logger) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockedService) SetMetricHandler(*monkit.Registry) error {
	args := m.Called()
	return args.Error(0)
}

func TestMainSingleProcess(t *testing.T) {
	mockService := new(MockedService)
	mockService.On("SetLogger", mock.Anything).Return(nil)
	mockService.On("SetMetricHandler", mock.Anything).Return(nil)
	mockService.On("Process", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	assert.Nil(t, process.Main(func() error { return nil }, mockService))
	mockService.AssertExpectations(t)
}

func TestMainMultipleProcess(t *testing.T) {
	// TODO: Fix the async issues in this test
	// mockService1 := MockedService{}
	// mockService2 := MockedService{}

	// mockService1.On("SetLogger", mock.Anything).Return(nil)
	// mockService1.On("SetMetricHandler", mock.Anything).Return(nil)
	// mockService1.On("Process", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// mockService2.On("SetLogger", mock.Anything).Return(nil)
	// mockService2.On("SetMetricHandler", mock.Anything).Return(nil)
	// mockService2.On("Process", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// assert.Nil(t, process.Main(func() error { return nil }, &mockService1, &mockService2))
	// mockService1.AssertExpectations(t)
	// mockService2.AssertExpectations(t)
	t.Skip()
}

func TestMainProcessError(t *testing.T) {
	mockService := MockedService{}

	err := process.ErrLogger.New("Process Error")
	mockService.On("SetLogger", mock.Anything).Return(nil)
	mockService.On("SetMetricHandler", mock.Anything).Return(nil)
	mockService.On("Process", mock.Anything, mock.Anything, mock.Anything).Return(err)
	assert.Equal(t, err, process.Main(func() error { return nil }, &mockService))
	mockService.AssertExpectations(t)
}

func TestConfigEnvironment(t *testing.T) {
	t.Skip()
}

func TestMust(t *testing.T) {
	t.Skip()
}

func TestExecute(t *testing.T) {
	t.Skip()
}
