// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package partners

import (
	"context"
)

// Partner refers to partners that
type Partner struct {
	ID   string
	Name string
}

// DB implements the database for partner information.
//
// architecture: Database
type DB interface {
	Add(ctx context.Context, partner Partner) error
	ByID(ctx context.Context, id string) (Partner, error)
	ByName(ctx context.Context, name string) ([]Partner, error)
	List(ctx context.Context) ([]Partner, error)
}
