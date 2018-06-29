package process

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
	return nil
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
	assert.Nil(t, process.Main(mockService))
}

func TestMainMultipleProcess(t *testing.T) {

}

func TestMust(t *testing.T) {
	t.Skip()
}

func TestExecute(t *testing.T) {
	t.Skip()
}
