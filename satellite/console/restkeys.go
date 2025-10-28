// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/restapikeys"
)

// NewRestKeysService returns a minimally set up console service that is set up ONLY
// for the purpose of managing REST API keys. This is particularly meant for Admin use.
func NewRestKeysService(log *zap.Logger, db restapikeys.DB, oauthRestKeysService restapikeys.Service, nowFn func() time.Time, config Config) restapikeys.Service {
	return &Service{
		log:           log,
		auditLogger:   log.Named("auditlog"),
		restKeys:      db,
		oauthRestKeys: oauthRestKeysService,
		config:        config,
		nowFn:         nowFn,
	}
}

// Create creates and inserts a rest key into the db.
func (s *Service) Create(ctx context.Context, name string, expiration *time.Duration) (apiKey string, expiresAt *time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "create rest key")
	if err != nil {
		return "", nil, Error.Wrap(err)
	}

	if !s.config.UseNewRestKeysTable {
		return "", nil, Error.New("Use CreateNoAuth instead")
	}

	if user.IsFreeOrMember() {
		return "", nil, ErrNotPaidTier.New("Only Pro users have access to REST keys")
	}

	apiKey, hash, err := s.GenerateNewKey(ctx)
	if err != nil {
		return "", nil, Error.Wrap(err)
	}

	now := s.nowFn()
	if expiration != nil {
		if *expiration <= 0 {
			expiration = &s.config.RestAPIKeys.DefaultExpiration
		}

		e := now.Add(*expiration)
		expiresAt = &e
	}
	restKey := restapikeys.Key{
		UserID:    user.ID,
		Token:     hash,
		Name:      name,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	_, err = s.restKeys.Create(ctx, restKey)
	if err != nil {
		return "", nil, Error.Wrap(err)
	}
	return apiKey, expiresAt, nil
}

// CreateNoAuth creates and inserts a rest key into the db for a user.
func (s *Service) CreateNoAuth(ctx context.Context, userID uuid.UUID, expiration *time.Duration) (apiKey string, expiresAt *time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.UseNewRestKeysTable {
		return s.oauthRestKeys.CreateNoAuth(ctx, userID, expiration)
	}

	apiKey, hash, err := s.GenerateNewKey(ctx)
	if err != nil {
		return "", nil, Error.Wrap(err)
	}

	now := s.nowFn()
	if expiration != nil {
		if *expiration <= 0 {
			expiration = &s.config.RestAPIKeys.DefaultExpiration
		}

		e := now.Add(*expiration)
		expiresAt = &e
	}

	restKey := &restapikeys.Key{
		UserID:    userID,
		Token:     hash,
		Name:      fmt.Sprintf("auth key - %d", now.UnixNano()),
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	restKey, err = s.restKeys.Create(ctx, *restKey)
	if err != nil {
		return "", nil, Error.Wrap(err)
	}

	s.auditLog(ctx, "create rest key", &userID, "", zap.String("keyID", restKey.ID.String()))

	return apiKey, expiresAt, nil
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

// GetUserAndExpirationFromKey gets the userID and expiration date attached to an account management api key.
func (s *Service) GetUserAndExpirationFromKey(ctx context.Context, apiKey string) (userID uuid.UUID, exp time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.UseNewRestKeysTable {
		return s.oauthRestKeys.GetUserAndExpirationFromKey(ctx, apiKey)
	}

	hash, err := s.HashKey(ctx, apiKey)
	if err != nil {
		return uuid.UUID{}, time.Now(), err
	}
	keyInfo, err := s.restKeys.GetByToken(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.UUID{}, time.Now(), Error.Wrap(ErrInvalidKey.New("invalid account management api key"))
		}
		return uuid.UUID{}, time.Now(), err
	}

	if keyInfo.ExpiresAt != nil {
		exp = *keyInfo.ExpiresAt
	}
	return keyInfo.UserID, exp, err
}

// GetAll gets a list of REST keys for the user in context.
func (s *Service) GetAll(ctx context.Context) (keys []restapikeys.Key, err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.UseNewRestKeysTable {
		return s.oauthRestKeys.GetAll(ctx)
	}

	user, err := s.getUserAndAuditLog(ctx, "get all rest keys")
	if err != nil {
		return keys, Error.Wrap(err)
	}

	if user.IsFreeOrMember() {
		return keys, ErrNotPaidTier.New("Only Pro users have access to REST keys")
	}

	keys, err = s.restKeys.GetAll(ctx, user.ID)
	if err != nil {
		return keys, Error.Wrap(err)
	}

	return keys, nil
}

// RevokeByKeyNoAuth revokes an account management api key
// this is meant for Admin use.
func (s *Service) RevokeByKeyNoAuth(ctx context.Context, apiKey string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.UseNewRestKeysTable {
		return s.oauthRestKeys.RevokeByKeyNoAuth(ctx, apiKey)
	}

	hash, err := s.HashKey(ctx, apiKey)
	if err != nil {
		return Error.Wrap(err)
	}

	restKey, err := s.restKeys.GetByToken(ctx, hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Error.Wrap(ErrInvalidKey.New("invalid REST API key"))
		}
		return Error.Wrap(err)
	}

	s.auditLog(ctx, "revoke rest api key", nil, "", zap.String("keyID", restKey.ID.String()))

	err = s.restKeys.Revoke(ctx, restKey.ID)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// RevokeByIDs revokes an account management api key by ID.
func (s *Service) RevokeByIDs(ctx context.Context, ids []uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.UseNewRestKeysTable {
		return s.oauthRestKeys.RevokeByIDs(ctx, ids)
	}

	user, err := s.getUserAndAuditLog(ctx, "revoke rest key")
	if err != nil {
		return Error.Wrap(err)
	}

	if user.IsFreeOrMember() {
		return ErrNotPaidTier.New("Only Pro users have access to REST keys")
	}

	for _, id := range ids {
		err = s.restKeys.Revoke(ctx, id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return Error.Wrap(err)
		}
	}
	return nil
}
