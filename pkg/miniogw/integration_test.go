// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw_test

import (
	"context"
	"errors"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/s3client"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/satellite/console"
)

func TestUploadDownload(t *testing.T) {
	t.Skip("disable because, keeps stalling Travis intermittently")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 30, 0)
	assert.NoError(t, err)

	defer ctx.Check(planet.Shutdown)

	// add project to satisfy constraint
	project, err := planet.Satellites[0].DB.Console().Projects().Insert(context.Background(), &console.Project{
		Name: "testProject",
	})

	assert.NoError(t, err)

	apiKey := console.APIKey{}
	apiKeyInfo := console.APIKeyInfo{
		ProjectID: project.ID,
		Name:      "testKey",
	}

	// add api key to db
	_, err = planet.Satellites[0].DB.Console().APIKeys().Create(context.Background(), apiKey, apiKeyInfo)
	assert.NoError(t, err)

	err = flag.Set("pointer-db.auth.api-key", apiKey.String())
	assert.NoError(t, err)

	// bind default values to config
	var gwCfg miniogw.Config
	cfgstruct.Bind(&pflag.FlagSet{}, &gwCfg)

	// minio config directory
	gwCfg.Minio.Dir = ctx.Dir("minio")

	// addresses
	gwCfg.Server.Address = "127.0.0.1:7777"
	gwCfg.Client.OverlayAddr = planet.Satellites[0].Addr()
	gwCfg.Client.PointerDBAddr = planet.Satellites[0].Addr()

	// keys
	gwCfg.Client.APIKey = "apiKey"
	gwCfg.Enc.Key = "encKey"

	// redundancy
	gwCfg.RS.MinThreshold = 7
	gwCfg.RS.RepairThreshold = 8
	gwCfg.RS.SuccessThreshold = 9
	gwCfg.RS.MaxThreshold = 10

	planet.Start(ctx)

	time.Sleep(2 * time.Second)

	// create identity for gateway
	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	// setup and start gateway
	go func() {
		// TODO: this leaks the gateway server, however it shouldn't
		err := runGateway(ctx, gwCfg, zaptest.NewLogger(t), identity)
		if err != nil {
			t.Log(err)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	client, err := s3client.NewMinio(s3client.Config{
		S3Gateway:     gwCfg.Server.Address,
		Satellite:     planet.Satellites[0].Addr(),
		AccessKey:     gwCfg.Minio.AccessKey,
		SecretKey:     gwCfg.Minio.SecretKey,
		APIKey:        gwCfg.Client.APIKey,
		EncryptionKey: gwCfg.Enc.Key,
		NoSSL:         true,
	})
	assert.NoError(t, err)

	bucket := "bucket"

	err = client.MakeBucket(bucket, "")
	assert.NoError(t, err)

	// generate enough data for a remote segment
	data := []byte{}
	for i := 0; i < 5000; i++ {
		data = append(data, 'a')
	}

	objectName := "testdata"

	err = client.Upload(bucket, objectName, data)
	assert.NoError(t, err)

	buffer := make([]byte, len(data))

	bytes, err := client.Download(bucket, objectName, buffer)
	assert.NoError(t, err)

	assert.Equal(t, string(data), string(bytes))
}

// runGateway creates and starts a gateway
func runGateway(ctx context.Context, c miniogw.Config, log *zap.Logger, identity *identity.FullIdentity) (err error) {

	// set gateway flags
	flags := flag.NewFlagSet("gateway", flag.ExitOnError)
	flags.String("address", c.Server.Address, "")
	flags.String("config-dir", c.Minio.Dir, "")
	flags.Bool("quiet", true, "")

	// create *cli.Context with gateway flags
	cliCtx := cli.NewContext(cli.NewApp(), flags, nil)

	// TODO: setting the flag on flagset and cliCtx seems redundant, but output is not quiet otherwise
	err = cliCtx.Set("quiet", "true")
	if err != nil {
		return err
	}

	err = os.Setenv("MINIO_ACCESS_KEY", c.Minio.AccessKey)
	if err != nil {
		return err
	}

	err = os.Setenv("MINIO_SECRET_KEY", c.Minio.SecretKey)
	if err != nil {
		return err
	}

	gw, err := c.NewGateway(ctx, identity)
	if err != nil {
		return err
	}

	minio.StartGateway(cliCtx, miniogw.Logging(gw, log))
	return errors.New("unexpected minio exit")
}
