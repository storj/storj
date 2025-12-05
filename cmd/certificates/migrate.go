// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/certificate/authorization"
)

var (
	migrateCmd = &cobra.Command{
		Use:   "migrate-gob",
		Short: "Migrate from gob encoding to protobuf encoding",
		RunE:  cmdMigrate,
	}
)

func cmdMigrate(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	authorizationDB, err := authorization.OpenDBFromCfg(ctx, runCfg.AuthorizationDB)
	if err != nil {
		return errs.New("error opening authorizations database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, authorizationDB.Close())
	}()

	log := zap.L()
	count, err := authorizationDB.MigrateGob(ctx, func(count int) {
		if count%100 == 0 {
			log.Info("progress", zap.Int("count", count))
		}
	})

	msg := "migration complete"
	if err != nil {
		msg = "migration interrupted"
	}
	log.Info(msg, zap.Int("processed", count))

	return err
}
