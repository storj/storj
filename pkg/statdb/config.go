// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package statdb

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/provider"
)

//CtxKey Used as statdb key
type CtxKey int

const (
	ctxKeyStats CtxKey = iota
)

// Config is a configuration struct that is everything you need to start a
// StatDB responsibility
type Config struct {
	DatabaseURL    string `help:"the database connection string to use" default:"$CONFDIR/stats.db"`
	DatabaseDriver string `help:"the database driver to use" default:"sqlite3"`
}

// Run implements the provider.Responsibility interface
func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	ns, err := NewStatDB(c.DatabaseDriver, c.DatabaseURL, zap.L())
	if err != nil {
		return err
	}

	return server.Run(context.WithValue(ctx, ctxKeyStats, ns))
}

// LoadFromContext loads an existing StatDB from the Provider context
// stack if one exists.
func LoadFromContext(ctx context.Context) *StatDB {
	if v, ok := ctx.Value(ctxKeyStats).(*StatDB); ok {
		return v
	}
	return nil
}
