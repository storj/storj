// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/cfgstruct"
	"storj.io/common/process"
	"storj.io/storj/private/revocation"
)

var (
	revocationsCmd = &cobra.Command{
		Use:         "revocations [service]",
		Short:       "Print revocation information from a revocation database",
		Args:        cobra.MaximumNArgs(1),
		RunE:        cmdRevocations,
		Annotations: map[string]string{"type": "setup"},
	}

	revCfg struct {
		RevocationDBURL string `default:"bolt://$CONFDIR/revocations.db" help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6379?db=2&password=abc123)"`
	}
)

func init() {
	rootCmd.AddCommand(revocationsCmd)

	process.Bind(revocationsCmd, &revCfg, defaults, cfgstruct.ConfDir(defaultConfigDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdRevocations(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	if len(args) > 0 {
		revCfg.RevocationDBURL = "bolt://" + filepath.Join(configDir, args[0], "revocations.db")
	}
	revDB, err := revocation.OpenDB(ctx, revCfg.RevocationDBURL)
	if err != nil {
		return err
	}

	revs, err := revDB.List(ctx)
	if err != nil {
		return err
	}

	revErrs := new(errs.Group)
	for _, rev := range revs {
		fmt.Printf("certificate public key hash: %s\n", base64.StdEncoding.EncodeToString(rev.KeyHash))
		fmt.Printf("\timestamp: %s\n", time.Unix(rev.Timestamp, 0).String())
		fmt.Printf("\tsignature: %s\n", base64.StdEncoding.EncodeToString(rev.Signature))
	}
	return revErrs.Err()
}
