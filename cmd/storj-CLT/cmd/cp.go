// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/process"
)

type config struct {
	miniogw.Config
}

var (
	cfg   config
	cpCmd = &cobra.Command{
		Use:   "cp",
		Short: "A brief description of your command",
		RunE:  copy,
	}
)

func init() {
	defaultConfDir := "$HOME/.storj/clt"

	RootCmd.AddCommand(cpCmd)
	cfgstruct.Bind(cpCmd.Flags(), &cfg, cfgstruct.ConfDir(defaultConfDir))
	cpCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func copy(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return errs.New("No file specified for copy")
	}

	if len(args) == 1 {
		return errs.New("No destination specified")
	}

	//TODO: actually get the proper config
	identity, err := cfg.LoadIdentity()
	if err != nil {
		return err
	}

	gateway, err := cfg.NewGateway(ctx, identity)
	if err != nil {
		return err
	}

	credentials, err := auth.CreateCredentials(cfg.AccessKey, cfg.SecretKey)
	if err != nil {
		return err
	}

	storjObjects, err := gateway.NewGatewayLayer(credentials)
	if err != nil {
		return err
	}

	sourceFile, err := os.Open(args[0])
	if err != nil {
		return err
	}

	fileInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	fileReader, err := hash.NewReader(sourceFile, fileInfo.Size(), "", "")
	if err != nil {
		return err
	}

	defer sourceFile.Close()

	destFile, err := url.Parse(args[1])
	if err != nil {
		return err
	}

	objInfo, err := storjObjects.PutObject(ctx, destFile.Host, destFile.Path, fileReader, nil)
	if err != nil {
		return err
	}

	fmt.Println("Bucket:", objInfo.Bucket)
	fmt.Println("Object:", objInfo.Name)

	return nil
}
