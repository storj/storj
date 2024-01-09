// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"sync"

	"github.com/zeebo/errs"

	"storj.io/common/lrucache"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensures that ConsoleDB implements console.DB.
var _ console.DB = (*ConsoleDB)(nil)

// ConsoleDB contains access to different satellite databases.
type ConsoleDB struct {
	apikeysLRUOptions lrucache.Options

	db *satelliteDB
	tx *dbx.Tx

	methods dbx.Methods

	apikeysOnce *sync.Once
	apikeys     *apikeys
}

// Users is getter a for Users repository.
func (db *ConsoleDB) Users() console.Users {
	return &users{db.db}
}

// Projects is a getter for Projects repository.
func (db *ConsoleDB) Projects() console.Projects {
	return &projects{db: db.methods, sdb: db.db}
}

// ProjectMembers is a getter for ProjectMembers repository.
func (db *ConsoleDB) ProjectMembers() console.ProjectMembers {
	return &projectMembers{db.methods, db.db}
}

// ProjectInvitations is a getter for ProjectInvitations repository.
func (db *ConsoleDB) ProjectInvitations() console.ProjectInvitations {
	return &projectInvitations{db.methods}
}

// APIKeys is a getter for APIKeys repository.
func (db *ConsoleDB) APIKeys() console.APIKeys {
	db.apikeysOnce.Do(func() {
		options := db.apikeysLRUOptions
		options.Name = "satellitedb-apikeys"
		db.apikeys = &apikeys{
			methods: db.methods,
			lru:     lrucache.NewOf[*dbx.ApiKey_Project_PublicId_Project_RateLimit_Project_BurstLimit_Project_SegmentLimit_Project_UsageLimit_Project_BandwidthLimit_Row](options),
			db:      db.db,
		}
	})

	return db.apikeys
}

// RegistrationTokens is a getter for RegistrationTokens repository.
func (db *ConsoleDB) RegistrationTokens() console.RegistrationTokens {
	return &registrationTokens{db.methods}
}

// ResetPasswordTokens is a getter for ResetPasswordTokens repository.
func (db *ConsoleDB) ResetPasswordTokens() console.ResetPasswordTokens {
	return &resetPasswordTokens{db.methods}
}

// WebappSessions is a getter for WebappSessions repository.
func (db *ConsoleDB) WebappSessions() consoleauth.WebappSessions {
	return &webappSessions{db.db}
}

// AccountFreezeEvents is a getter for AccountFreezeEvents repository.
func (db *ConsoleDB) AccountFreezeEvents() console.AccountFreezeEvents {
	return &accountFreezeEvents{db.db}
}

// WithTx is a method for executing and retrying transaction.
func (db *ConsoleDB) WithTx(ctx context.Context, fn func(context.Context, console.DBTx) error) error {
	if db.db == nil {
		return errs.New("DB is not initialized!")
	}

	return db.db.WithTx(ctx, func(ctx context.Context, tx *dbx.Tx) error {
		dbTx := &DBTx{
			ConsoleDB: &ConsoleDB{
				apikeysLRUOptions: db.apikeysLRUOptions,

				// Need to expose dbx.DB for when database methods need access to check database driver type
				db:      db.db,
				tx:      tx,
				methods: tx,

				apikeysOnce: db.apikeysOnce,
				apikeys:     db.apikeys,
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
