package testing_utils

import (
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
)

type MockGateway struct {
	Ol minio.ObjectLayer
}

func (gw *MockGateway) Name() string {
	return "MockGateway"
}

func (gw *MockGateway) Production() bool {
	return false
}

func (gw *MockGateway) NewGatewayLayer(creds auth.Credentials) (objLayer minio.ObjectLayer, err error) {
	return gw.Ol, nil
}
