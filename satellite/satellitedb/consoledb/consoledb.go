// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"context"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/lrucache"
)

// ensures that ConsoleDB implements console.DB.
var _ console.DB = (*ConsoleDB)(nil)

// ConsoleDB contains access to different satellite databases.
type ConsoleDB struct {
	*dbx.DB

	ApikeysLRUOptions lrucache.Options

	Impl dbutil.Implementation
	tx   *dbx.Tx

	Methods dbx.DriverMethods

	ApikeysOnce  *sync.Once
	apikeysCache *lrucache.ExpiringLRUOf[*projectApiKeyRow]
}

// Users is getter a for Users repository.
func (db *ConsoleDB) Users() console.Users {
	return &users{db: db.Methods, impl: db.Impl}
}

// Projects is a getter for Projects repository.
func (db *ConsoleDB) Projects() console.Projects {
	return &projects{db: db.Methods}
}

// ProjectMembers is a getter for ProjectMembers repository.
func (db *ConsoleDB) ProjectMembers() console.ProjectMembers {
	return &projectMembers{db: db.Methods, impl: db.Impl}
}

// ProjectInvitations is a getter for ProjectInvitations repository.
func (db *ConsoleDB) ProjectInvitations() console.ProjectInvitations {
	return &projectInvitations{db: db.Methods}
}

// APIKeys is a getter for APIKeys repository.
func (db *ConsoleDB) APIKeys() console.APIKeys {
	db.ApikeysOnce.Do(func() {
		options := db.ApikeysLRUOptions
		options.Name = "satellitedb-apikeys"
		db.apikeysCache = lrucache.NewOf[*projectApiKeyRow](options)
	})

	return &apikeys{
		db:   db.Methods,
		lru:  db.apikeysCache,
		impl: db.Impl,
	}
}

// RegistrationTokens is a getter for RegistrationTokens repository.
func (db *ConsoleDB) RegistrationTokens() console.RegistrationTokens {
	return &registrationTokens{db: db.Methods}
}

// ResetPasswordTokens is a getter for ResetPasswordTokens repository.
func (db *ConsoleDB) ResetPasswordTokens() console.ResetPasswordTokens {
	return &resetPasswordTokens{db: db.Methods}
}

// WebappSessions is a getter for WebappSessions repository.
func (db *ConsoleDB) WebappSessions() consoleauth.WebappSessions {
	return &webappSessions{db: db.Methods, impl: db.Impl}
}

// AccountFreezeEvents is a getter for AccountFreezeEvents repository.
func (db *ConsoleDB) AccountFreezeEvents() console.AccountFreezeEvents {
	return &accountFreezeEvents{db: db.Methods}
}

// WithTx is a method for executing and retrying transaction.
func (db *ConsoleDB) WithTx(ctx context.Context, fn func(context.Context, console.DBTx) error) error {
	if db.DB == nil {
		return errs.New("DB is not initialized!")
	}

	return db.DB.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		dbTx := &DBTx{
			ConsoleDB: &ConsoleDB{
				ApikeysLRUOptions: db.ApikeysLRUOptions,

				// Need to expose dbx.DB for when database Methods need access to check database driver type
				DB:      db.DB,
				tx:      tx,
				Methods: tx,

				ApikeysOnce:  db.ApikeysOnce,
				apikeysCache: db.apikeysCache,
			},
		}
		return fn(ctx, dbTx)
	})
}

// DBTx extends Database with transaction scope.
type DBTx struct {
	*ConsoleDB
}

// Commit is a method for committing and closing transaction.
func (db *DBTx) Commit() error {
	if db.tx == nil {
		return errs.New("begin transaction before commit it!")
	}

	return db.tx.Commit()
}

// Rollback is a method for rollback and closing transaction.
func (db *DBTx) Rollback() error {
	if db.tx == nil {
		return errs.New("begin transaction before rollback it!")
	}

	return db.tx.Rollback()
}
