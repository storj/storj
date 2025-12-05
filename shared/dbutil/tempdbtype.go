// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"github.com/zeebo/errs"

	"storj.io/storj/shared/tagsql"
)

// TempDatabase is a database (or something that works like an isolated database,
// such as a PostgreSQL schema) with a semi-unique name which will be cleaned up
// when closed. Mainly useful for testing purposes.
type TempDatabase struct {
	tagsql.DB
	ConnStr        string
	Schema         string
	Driver         string
	Implementation Implementation
	Cleanup        func(tagsql.DB) error
}

// Close closes the database and deletes the schema.
func (db *TempDatabase) Close() error {
	return errs.Combine(
		db.Cleanup(db.DB),
		db.DB.Close(),
	)
}
