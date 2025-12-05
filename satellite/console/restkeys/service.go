// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package restkeys

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/oidc"
)

var mon = monkit.Package()

var (
	// Error describes internal rest keys error.
	Error = errs.Class("rest keys service")

	// ErrDuplicateKey is error type that occurs when a generated account
	// management api key already exists.
	ErrDuplicateKey = errs.Class("duplicate key")

	// ErrInvalidKey is an error type that occurs when a user submits a key
	// that does not match anything in the database.
	ErrInvalidKey = errs.Class("invalid key")
)

// Config contains configuration parameters for rest keys.
type Config struct {
	DefaultExpiration time.Duration `help:"expiration to use if user does not specify an rest key expiration" default:"720h"`
}

// Service handles operations regarding rest keys.
type Service struct {
	db                oidc.OAuthTokens
	defaultExpiration time.Duration
}

// NewService creates a new rest keys service.
func NewService(db oidc.OAuthTokens, defaultExpiration time.Duration) *Service {
	return &Service{
		db:                db,
		defaultExpiration: defaultExpiration,
	}
}

// CreateNoAuth creates and inserts a rest key into the db for a user.
func (s *Service) CreateNoAuth(ctx context.Context, userID uuid.UUID, expiration *time.Duration) (apiKey string, expiresAt *time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	if expiration == nil {
		expiration = &s.defaultExpiration
	}

	apiKey, hash, err := s.GenerateNewKey(ctx)
	if err != nil {
		return "", nil, Error.Wrap(err)
	}
	e, err := s.InsertIntoDB(ctx, oidc.OAuthToken{
		UserID: userID,
		Kind:   oidc.KindRESTTokenV0,
		Token:  hash,
	}, time.Now(), *expiration)
	if err != nil {
		return "", nil, Error.Wrap(err)
	}
	return apiKey, &e, nil
}

// Create creates and inserts a rest key into the db.
func (s *Service) Create(ctx context.Context, name string, expiration *time.Duration) (apiKey string, expiresAt *time.Time, err error) {
	return "", nil, Error.New("Use CreateNoAuth instead")
}

// GenerateNewKey generates a new account management api key.
func (s *Service) GenerateNewKey(ctx context.Context) (apiKey, hash string, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := uuid.New()
	if err != nil {
		return "", "", Error.Wrap(err)
	}

	apiKey = id.String()
	hash = hashKeyFromUUID(ctx, id)
	return apiKey, hash, nil
}

// This is used for hashing during key creation so we don't need to convert from a string back to a uuid.
func hashKeyFromUUID(ctx context.Context, apiKeyUUID uuid.UUID) string {
	mon.Task()(&ctx)(nil)

	hashBytes := sha256.Sum256(apiKeyUUID.Bytes())
	return string(hashBytes[:])
}

// HashKey returns a hash of api key. This is used for hashing inside GetUserFromKey.
func (s *Service) HashKey(ctx context.Context, apiKey string) (hash string, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := uuid.FromString(apiKey)
	if err != nil {
		return "", Error.Wrap(err)
	}
	hashBytes := sha256.Sum256(id.Bytes())
	return string(hashBytes[:]), nil
}

// InsertIntoDB checks OAuthTokens DB for a token before inserting. This is because OAuthTokens DB allows
// duplicate tokens, but we can't have duplicate api keys.
func (s *Service) InsertIntoDB(ctx context.Context, oAuthToken oidc.OAuthToken, now time.Time, expiration time.Duration) (expiresAt time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	// The token column is the key to the OAuthTokens table, but the Create method does not return an error if a duplicate token insert is attempted.
	// We need to make sure a unique api key is created, so check that the value doesn't already exist.
	_, err = s.db.Get(ctx, oidc.KindRESTTokenV0, oAuthToken.Token)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, Error.Wrap(err)
		}
	} else if err == nil {
		return time.Time{}, Error.Wrap(ErrDuplicateKey.New("failed to generate a unique account management api key"))
	}

	if expiration <= 0 {
		expiration = s.defaultExpiration
	}
	expiresAt = now.Add(expiration)

	oAuthToken.CreatedAt = now
	oAuthToken.ExpiresAt = expiresAt

	err = s.db.Create(ctx, oAuthToken)
	if err != nil {
		return time.Time{}, Error.Wrap(err)
	}
	return expiresAt, nil
}

// GetUserAndExpirationFromKey gets the userID and expiration date attached to an account management api key.
func (s *Service) GetUserAndExpirationFromKey(ctx context.Context, apiKey string) (userID uuid.UUID, exp time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	hash, err := s.HashKey(ctx, apiKey)
	if err != nil {
		return uuid.UUID{}, time.Now(), err
	}
	keyInfo, err := s.db.Get(ctx, oidc.KindRESTTokenV0, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, time.Now(), Error.Wrap(ErrInvalidKey.New("invalid account management api key"))
		}
		return uuid.UUID{}, time.Now(), err
	}
	return keyInfo.UserID, keyInfo.ExpiresAt, err
}

// Revoke revokes an account management api key.
func (s *Service) Revoke(ctx context.Context, apiKey string) (err error) {
	defer mon.Task()(&ctx)(&err)

	hash, err := s.HashKey(ctx, apiKey)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = s.db.Get(ctx, oidc.KindRESTTokenV0, hash)
	if err != nil {
		return Error.Wrap(err)
	}
	err = s.db.RevokeRESTTokenV0(ctx, hash)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// RevokeByKeyNoAuth revokes an account management api key
// this is meant for Admin use.
func (s *Service) RevokeByKeyNoAuth(ctx context.Context, apiKey string) (err error) {
	return s.Revoke(ctx, apiKey)
}

// RevokeByIDs revokes an account management api key by ID.
func (s *Service) RevokeByIDs(ctx context.Context, ids []uuid.UUID) (err error) {
	return Error.New("RevokeByIDs is not implemented")
}

// GetAll gets a list of REST keys for the user in context.
func (s *Service) GetAll(ctx context.Context) ([]restapikeys.Key, error) {
	return nil, Error.New("GetAll is not implemented")
}
