// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw_test

import (
	"context"
	"errors"
	"flag"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/s3client"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/testrand"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/macaroon"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/console"
	"storj.io/storj/uplink"
)

type config struct {
	Server miniogw.ServerConfig
	Minio  miniogw.MinioConfig
}

func TestUploadDownload(t *testing.T) {
	t.Skip("disable because, keeps stalling CI intermittently")

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

	apiKey, err := macaroon.NewAPIKey([]byte("testSecret"))
	assert.NoError(t, err)

	apiKeyInfo := console.APIKeyInfo{
		ProjectID: project.ID,
		Name:      "testKey",
		Secret:    []byte("testSecret"),
	}

	// add api key to db
	_, err = planet.Satellites[0].DB.Console().APIKeys().Create(context.Background(), apiKey.Head(), apiKeyInfo)
	assert.NoError(t, err)

	// bind default values to config
	var gwCfg config
	cfgstruct.Bind(&pflag.FlagSet{}, &gwCfg, cfgstruct.UseDevDefaults())
	var uplinkCfg uplink.Config
	cfgstruct.Bind(&pflag.FlagSet{}, &uplinkCfg, cfgstruct.UseDevDefaults())

	// minio config directory
	gwCfg.Minio.Dir = ctx.Dir("minio")

	// addresses
	gwCfg.Server.Address = "127.0.0.1:7777"
	uplinkCfg.Client.SatelliteAddr = planet.Satellites[0].Addr()

	// keys
	uplinkCfg.Client.APIKey = "apiKey"

	// Encryption key
	passphrase := testrand.BytesInt(testrand.Intn(100) + 1)

	encryptionKey, err := storj.NewKey(passphrase)
	require.NoError(t, err)
	filename := ctx.File("encryption.key")
	err = ioutil.WriteFile(filename, encryptionKey[:], os.FileMode(0400))
	require.NoError(t, err)
	uplinkCfg.Enc.KeyFilepath = filename

	// redundancy
	uplinkCfg.RS.MinThreshold = 7
	uplinkCfg.RS.RepairThreshold = 8
	uplinkCfg.RS.SuccessThreshold = 9
	uplinkCfg.RS.MaxThreshold = 10

	planet.Start(ctx)

	// create identity for gateway
	ca, err := testidentity.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	// setup and start gateway
	go func() {
		// TODO: this leaks the gateway server, however it shouldn't
		err := runGateway(ctx, gwCfg, uplinkCfg, zaptest.NewLogger(t), identity)
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
		APIKey:        uplinkCfg.Client.APIKey,
		EncryptionKey: string(encryptionKey[:]),
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
func runGateway(ctx context.Context, gwCfg config, uplinkCfg uplink.Config, log *zap.Logger, ident *identity.FullIdentity) (err error) {

	// set gateway flags
	flags := flag.NewFlagSet("gateway", flag.ExitOnError)
	flags.String("address", gwCfg.Server.Address, "")
	flags.String("config-dir", gwCfg.Minio.Dir, "")
	flags.Bool("quiet", true, "")

	// create *cli.Context with gateway flags
	cliCtx := cli.NewContext(cli.NewApp(), flags, nil)

	// TODO: setting the flag on flagset and cliCtx seems redundant, but output is not quiet otherwise
	err = cliCtx.Set("quiet", "true")
	if err != nil {
		return err
	}

	err = os.Setenv("MINIO_ACCESS_KEY", gwCfg.Minio.AccessKey)
	if err != nil {
		return err
	}

	err = os.Setenv("MINIO_SECRET_KEY", gwCfg.Minio.SecretKey)
	if err != nil {
		return err
	}

	cfg := libuplink.Config{}
	cfg.Volatile.TLS = struct {
		SkipPeerCAWhitelist bool
		PeerCAWhitelistPath string
	}{
		SkipPeerCAWhitelist: !uplinkCfg.TLS.UsePeerCAWhitelist,
		PeerCAWhitelistPath: uplinkCfg.TLS.PeerCAWhitelistPath,
	}
	cfg.Volatile.MaxInlineSize = uplinkCfg.Client.MaxInlineSize
	cfg.Volatile.MaxMemory = uplinkCfg.RS.MaxBufferMem

	uplink, err := libuplink.NewUplink(ctx, &cfg)
	if err != nil {
		return err
	}

	apiKey, err := libuplink.ParseAPIKey(uplinkCfg.Client.APIKey)
	if err != nil {
		return err
	}

	project, err := uplink.OpenProject(ctx, uplinkCfg.Client.SatelliteAddr, apiKey)
	if err != nil {
		return err
	}

	gw := miniogw.NewStorjGateway(
		project,
		&storj.Key{},
		storj.Cipher(uplinkCfg.Enc.PathType).ToCipherSuite(),
		uplinkCfg.GetEncryptionScheme().ToEncryptionParameters(),
		uplinkCfg.GetRedundancyScheme(),
		uplinkCfg.Client.SegmentSize,
	)

	minio.StartGateway(cliCtx, miniogw.Logging(gw, log))
	return errors.New("unexpected minio exit")
}
