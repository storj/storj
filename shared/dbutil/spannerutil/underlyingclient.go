// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"

	"cloud.google.com/go/spanner"
	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

// UnderlyingClient implements exposing *spanner.Client from a tagsql.DB.
func UnderlyingClient(ctx context.Context, db tagsql.DB, fn func(client *spanner.Client) error) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, errs.Wrap(conn.Close())) }()

	return errs.Wrap(conn.Raw(ctx, func(driverConn interface{}) error {
		spannerConn, ok := driverConn.(interface {
			UnderlyingClient() (*spanner.Client, error)
		})
		if !ok {
			return errs.New("expected driver to have UnderlyingClient, but had type %T", driverConn)
		}

		client, err := spannerConn.UnderlyingClient()
		if err != nil {
			return errs.Wrap(err)
		}

		return errs.Wrap(fn(client))
	}))
}
