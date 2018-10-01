package gateway

import (
	"github.com/minio/cli"
	"github.com/minio/minio/pkg/auth"
	"storj.io/mirroring/pkg/config"
	"storj.io/mirroring/pkg/object_layer/mirroring"
	"storj.io/mirroring/utils"

	minio "github.com/minio/minio/cmd"
	s3 "storj.io/mirroring/pkg/object_layer/s3compat"
	"errors"
)

func init() {
	err := minio.RegisterGatewayCommand(cli.Command{
		Name:            "mirroring",
		Usage:           "mirroring",
		Action:          mirroringGatewayMain,
		HideHelpCommand: true,
	})

	if err != nil {
	}
}

func mirroringGatewayMain(ctx *cli.Context) {
	minio.StartGateway(ctx, &Mirroring{})
}

// Mirroring for mirroring service
type Mirroring struct {
	Config *config.Config
	Logger utils.Logger
}

// Name implements minio.Gateway interface
func (gw *Mirroring) Name() string {
	return ""
}

// NewGatewayLayer implements minio.Gateway interface
func (gw *Mirroring) NewGatewayLayer(creds auth.Credentials) (objLayer minio.ObjectLayer, err error) {
	if gw.Config == nil {

		return nil, errors.New("configuration is not set")
	}

	s1Credentials := gw.Config.Server1
	prime, err := s3.NewS3Compat(s1Credentials.Endpoint, s1Credentials.AccessKey, s1Credentials.SecretKey)

	if err != nil {
		return nil, err
	}

	s2Credentials := gw.Config.Server2
	alter, err := s3.NewS3Compat(s2Credentials.Endpoint, s2Credentials.AccessKey, s2Credentials.SecretKey)

	if err != nil {
		return nil, err
	}

	objLayer = &mirroring.MirroringObjectLayer{
		Prime:  prime,
		Alter:  alter,
		Logger: gw.Logger,
	}

	return objLayer, nil
}

// Production - both gateways are production ready.
func (gw *Mirroring) Production() bool {
	return false
}
