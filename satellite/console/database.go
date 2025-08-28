// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/entitlements"
)

// DB contains access to different satellite databases.
//
// architecture: Database
type DB interface {
	// Users is a getter for Users repository.
	Users() Users
	// Projects is a getter for Projects repository.
	Projects() Projects
	// ProjectMembers is a getter for ProjectMembers repository.
	ProjectMembers() ProjectMembers
	// ProjectInvitations is a getter for ProjectInvitations repository.
	ProjectInvitations() ProjectInvitations
	// APIKeys is a getter for APIKeys repository.
	APIKeys() APIKeys
	// RestApiKeys returns the database for REST API keys.
	RestApiKeys() restapikeys.DB
	// RegistrationTokens is a getter for RegistrationTokens repository.
	RegistrationTokens() RegistrationTokens
	// ResetPasswordTokens is a getter for ResetPasswordTokens repository.
	ResetPasswordTokens() ResetPasswordTokens
	// WebappSessions is a getter for WebappSessions repository.
	WebappSessions() consoleauth.WebappSessions
	// AccountFreezeEvents is a getter for AccountFreezeEvents repository.
	AccountFreezeEvents() AccountFreezeEvents
	// APIKeyTails is a getter for APIKeyTails repository.
	APIKeyTails() APIKeyTails
	// Domains is a getter for Domains repository.
	Domains() Domains
	// Entitlements is a getter for Entitlements repository.
	Entitlements() entitlements.DB

	// WithTx is a method for executing transactions with retrying as necessary.
	WithTx(ctx context.Context, fn func(ctx context.Context, tx DBTx) error) error
}

// DBTx extends Database with transaction scope.
type DBTx interface {
	DB
	// Commit is a method for committing and closing transaction.
	Commit() error
	// Rollback is a method for rollback and closing transaction.
	Rollback() error
}
