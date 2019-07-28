// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"storj.io/storj/satellite/accounting"
)

// DB contains access to different satellite databases
type DB interface {
	// Users is a getter for Users repository
	Users() Users
	// Projects is a getter for Projects repository
	Projects() Projects
	// ProjectMembers is a getter for ProjectMembers repository
	ProjectMembers() ProjectMembers
	// APIKeys is a getter for APIKeys repository
	APIKeys() APIKeys
	// BucketUsage is a getter for accounting.BucketUsage repository
	BucketUsage() accounting.BucketUsage
	// RegistrationTokens is a getter for RegistrationTokens repository
	RegistrationTokens() RegistrationTokens
	// ResetPasswordTokens is a getter for ResetPasswordTokens repository
	ResetPasswordTokens() ResetPasswordTokens
	// UsageRollups is a getter for UsageRollups repository
	UsageRollups() UsageRollups
	// UserCredits is a getter for UserCredits repository
	UserCredits() UserCredits
	// UserPayments is a getter for UserPayments repository
	UserPayments() UserPayments
	// ProjectPayments is a getter for ProjectPayments repository
	ProjectPayments() ProjectPayments
	// ProjectInvoiceStamps is a getter for ProjectInvoiceStamps repository
	ProjectInvoiceStamps() ProjectInvoiceStamps

	// BeginTransaction is a method for opening transaction
	BeginTx(ctx context.Context) (DBTx, error)
}

// DBTx extends Database with transaction scope
type DBTx interface {
	DB
	// CommitTransaction is a method for committing and closing transaction
	Commit() error
	// RollbackTransaction is a method for rollback and closing transaction
	Rollback() error
}
