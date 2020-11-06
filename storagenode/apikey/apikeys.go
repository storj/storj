// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package apikey

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/zeebo/errs"
)

// ErrNoSecret represents errors from the apikey database.
var ErrNoSecret = errs.Class("no apikey error")

// DB is interface for working with apikey tokens.
//
// architecture: Database
type DB interface {
	// Store stores apikey token into db.
	Store(ctx context.Context, secret APIKey) error

	// Check checks if unique apikey exists in db by token.
	Check(ctx context.Context, token Secret) error

	// Revoke removes token from db.
	Revoke(ctx context.Context, token Secret) error
}

// Secret stores token of storagenode APIkey.
type Secret [32]byte

// APIKey describing apikey model in the database.
type APIKey struct {
	// Secret is PK of the table and keeps unique value sno apikey token
	Secret Secret

	CreatedAt time.Time `json:"createdAt"`
}

// NewSecretToken creates new apikey token.
func NewSecretToken() (Secret, error) {
	var b [32]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return b, errs.New("error creating apikey token")
	}

	return b, nil
}

// String implements Stringer.
func (secret Secret) String() string {
	return base64.URLEncoding.EncodeToString(secret[:])
}

// IsZero returns if the apikey token is not set.
func (secret Secret) IsZero() bool {
	var zero Secret
	// this doesn't need to be constant-time, because we're explicitly testing
	// against a hardcoded, well-known value
	return bytes.Equal(secret[:], zero[:])
}

// TokenSecretFromBase64 creates new apikey token from base64 string.
func TokenSecretFromBase64(s string) (Secret, error) {
	var token Secret

	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return token, err
	}

	copy(token[:], b)

	return token, nil
}
