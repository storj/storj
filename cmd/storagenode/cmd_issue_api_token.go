// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/process"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/apikeys"
	"storj.io/storj/storagenode/storagenodedb"
)

type issueCfg struct {
	storagenode.Config
}

func newIssueAPIKeyCmd(f *Factory) *cobra.Command {
	var cfg issueCfg

	cmd := &cobra.Command{
		Use:   "issue-apikey",
		Short: "Issue a new api key",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdIssue(cmd, &cfg)
		},
	}

	process.Bind(cmd, &cfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func cmdIssue(cmd *cobra.Command, cfg *issueCfg) (err error) {
	ctx, _ := process.Ctx(cmd)

	ident, err := cfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	} else {
		zap.L().Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	db, err := storagenodedb.OpenExisting(ctx, zap.L().Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	service := apikeys.NewService(db.APIKeys())

	apiKey, err := service.Issue(ctx)
	if err != nil {
		return errs.New("Error while trying to issue new api key: %v", err)
	}

	fmt.Println(apiKey.Secret.String())

	return
}
