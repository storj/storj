// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package accounts

import (
	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/pkg/accounts/accountdb/dbo"
)

// DB used to manage db connections and context through different repositories
type DB interface {
	User() Users

	CreateTables() error
	Dispose() error
}

// Users exposes methods to manage User table in database.
type Users interface {
	GetByCredentials(password []byte, email string) (*dbo.User, error)
	Get(id uuid.UUID) (*dbo.User, error)
	Insert(user *dbo.User) (error)
	Delete(id uuid.UUID) (error)
	Update(user *dbo.User) (error)
}