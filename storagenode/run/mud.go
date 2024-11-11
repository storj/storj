// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"storj.io/storj/storagenode"
)

// Module registers all the possible components for the storagenode instance.
func Module(ball *mud.Ball) {
	mud.Provide[*zap.Logger](ball, func() (*zap.Logger, error) {
		logger, err := zap.NewDevelopment()
		if err != nil {
			return nil, errs.Wrap(err)
		}
		return logger.With(zap.String("Process", "storagenode")), nil
	})

	modular.IdentityModule(ball)
	storagenode.Module(ball)
}
