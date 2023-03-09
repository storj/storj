// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

import (
	"context"
	"time"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// DB defines a collection of resources that fall under the scope of OIDC and OAuth operations.
//
// architecture: Database
type DB interface {
	// OAuthClients returns an API for the oauthclients repository.
	OAuthClients() OAuthClients
	// OAuthCodes returns an API for the oauthcodes repository.
	OAuthCodes() OAuthCodes
	// OAuthTokens returns an API for the oauthtokens repository.
	OAuthTokens() OAuthTokens
}

// OAuthClients defines an interface for creating, updating, and obtaining information about oauth clients known to our
// system.
type OAuthClients interface {
	// Get returns the OAuthClient associated with the provided id.
	Get(ctx context.Context, id uuid.UUID) (OAuthClient, error)

	// Create creates a new OAuthClient.
	Create(ctx context.Context, client OAuthClient) error

	// Update modifies information for the provided OAuthClient.
	Update(ctx context.Context, client OAuthClient) error

	// Delete deletes the identified client from the database.
	Delete(ctx context.Context, id uuid.UUID) error
}

// OAuthClient defines a concrete representation of an oauth client.
type OAuthClient struct {
	ID          uuid.UUID `json:"id"`
	Secret      []byte    `json:"secret"`
	UserID      uuid.UUID `json:"userID"`
	RedirectURL string    `json:"redirectURL"`
	AppName     string    `json:"appName"`
	AppLogoURL  string    `json:"appLogoURL"`
}

// GetID returns the clients id.
func (o OAuthClient) GetID() string {
	return o.ID.String()
}

// GetSecret returns the clients secret.
func (o OAuthClient) GetSecret() string {
	return string(o.Secret)
}

// GetDomain returns the allowed redirect url associated with the client.
func (o OAuthClient) GetDomain() string {
	return o.RedirectURL
}

// GetUserID returns the owners' user id.
func (o OAuthClient) GetUserID() string {
	return o.UserID.String()
}

// OAuthCodes defines a set of operations allowed to be performed against oauth codes.
type OAuthCodes interface {
	// Get retrieves the OAuthCode for the specified code. Implementations should only return unexpired, unclaimed
	// codes. Once a code has been claimed, it should be marked as such to prevent future calls from exchanging the
	// value for an access tokens.
	Get(ctx context.Context, code string) (OAuthCode, error)

	// Create creates a new OAuthCode.
	Create(ctx context.Context, code OAuthCode) error

	// Claim marks that the provided code has been claimed and should not be issued to another caller.
	Claim(ctx context.Context, code string) error
}

// OAuthTokens defines a set of operations that ca be performed against oauth tokens.
type OAuthTokens interface {
	// Get retrieves the OAuthToken for the specified kind and token value. This can be used to look up either refresh
	// or access tokens that have not expired.
	Get(ctx context.Context, kind OAuthTokenKind, token string) (OAuthToken, error)

	// Create creates a new OAuthToken. If the token already exists, no value is modified and nil is returned.
	Create(ctx context.Context, token OAuthToken) error

	// RevokeRESTTokenV0 revokes a v0 rest token by setting its expires_at time to zero.
	RevokeRESTTokenV0(ctx context.Context, token string) error
}

// OAuthTokenKind defines an enumeration of different types of supported tokens.
type OAuthTokenKind int8

const (
	// KindUnknown is used to represent an entry for which we do not recognize the value.
	KindUnknown = 0
	// KindAccessToken represents an access token within the database.
	KindAccessToken = 1
	// KindRefreshToken represents a refresh token within the database.
	KindRefreshToken = 2
	// KindRESTTokenV0 represents a REST token within the database.
	KindRESTTokenV0 = 3
)

// OAuthCode represents a code stored within our database.
type OAuthCode struct {
	ClientID        uuid.UUID
	UserID          uuid.UUID
	Scope           string
	RedirectURL     string
	Challenge       string
	ChallengeMethod string
	Code            string
	CreatedAt       time.Time
	ExpiresAt       time.Time
	ClaimedAt       *time.Time
}

// OAuthToken represents a token stored within our database (either access / refresh).
type OAuthToken struct {
	ClientID  uuid.UUID
	UserID    uuid.UUID
	Scope     string
	Kind      OAuthTokenKind
	Token     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewDB constructs a database using the provided dbx db.
func NewDB(dbxdb *dbx.DB) DB {
	return &db{
		clients: &clientsDBX{
			db: dbxdb,
		},
		codes: &codesDBX{
			db: dbxdb,
		},
		tokens: &tokensDBX{
			db: dbxdb,
		},
	}
}

type db struct {
	clients OAuthClients
	codes   OAuthCodes
	tokens  OAuthTokens
}

func (d *db) OAuthClients() OAuthClients {
	return d.clients
}

func (d *db) OAuthCodes() OAuthCodes {
	return d.codes
}

func (d *db) OAuthTokens() OAuthTokens {
	return d.tokens
}

var _ DB = &db{}
