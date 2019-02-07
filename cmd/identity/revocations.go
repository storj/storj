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

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
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
		RevocationDBURL string `default:"bolt://$CONFDIR/revocations.db" help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)"`
	}
)

func init() {
	rootCmd.AddCommand(revocationsCmd)

	cfgstruct.Bind(revocationsCmd.Flags(), &revCfg, cfgstruct.ConfDir(defaultConfigDir), cfgstruct.IdentityDir(defaultIdentityDir))
}

func cmdRevocations(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		revCfg.RevocationDBURL = "bolt://" + filepath.Join(configDir, args[0], "revocations.db")
	}

	revDB, err := identity.NewRevDB(revCfg.RevocationDBURL)
	if err != nil {
		return err
	}

	revs, err := revDB.List()
	if err != nil {
		return err
	}

	revErrs := new(errs.Group)
	for _, rev := range revs {
		// TODO: figure out why this doesn't base64 encode []byte fields!
		//revJSON, err := json.MarshalIndent(rev.CertHash, "", "\t")
		//if err != nil {
		//	revErrs.Add(err)
		//}
		fmt.Printf("certificate hash: %s\n", base64.StdEncoding.EncodeToString(rev.CertHash))
		fmt.Printf("\timestamp: %s\n", time.Unix(rev.Timestamp, 0).String())
		fmt.Printf("\tsignature: %s\n", base64.StdEncoding.EncodeToString(rev.Signature))
	}
	return revErrs.Err()
}
