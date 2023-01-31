// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/process"
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
	count := 0
	return authorizationDB.MigrateGob(ctx, func(userID string) {
		if count%100 == 0 {
			log.Info("progress", zap.String("last", userID), zap.Int("total-processed-count", count))
		}
		count++
	})
}
