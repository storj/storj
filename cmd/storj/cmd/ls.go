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
	"storj.io/storj/pkg/process"
)

var (
	lsCfg Config
	lsCmd = &cobra.Command{
		Use:   "ls",
		Short: "A brief description of your command",
		RunE:  list,
	}
)

func init() {
	RootCmd.AddCommand(lsCmd)
	cfgstruct.Bind(lsCmd.Flags(), &lsCfg, cfgstruct.ConfDir(defaultConfDir))
	lsCmd.Flags().String("config", filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
}

func list(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return errs.New("No file specified for copy")
	}

	if len(args) == 1 {
		return errs.New("No destination specified")
	}

	//TODO: actually get the proper config
	identity, err := lsCfg.LoadIdentity()
	if err != nil {
		return err
	}

	gateway, err := lsCfg.NewGateway(ctx, identity)
	if err != nil {
		return err
	}

	credentials, err := auth.CreateCredentials(lsCfg.AccessKey, lsCfg.SecretKey)
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
