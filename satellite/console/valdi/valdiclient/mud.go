// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package valdiclient

import (
	"net/http"

	"go.uber.org/zap"

	"storj.io/storj/shared/mud"
)

// Module is a mud module.
func Module(ball *mud.Ball) {
	mud.Provide[*Client](ball, func(log *zap.Logger, config Config) (*Client, error) {
		return New(log, http.DefaultClient, config)
	})
	mud.Tag[*Client](ball, mud.Optional{})
	mud.Tag[*Client](ball, mud.Nullable{})
}
