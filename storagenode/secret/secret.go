// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package secret

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// ErrNoSecret represents errors from the secret database.
var ErrNoSecret = errs.Class("no secret error")

// DB is interface for working with secret tokens.
//
// architecture: Database
type DB interface {
	// Store stores secret token into db.
	Store(ctx context.Context, secret UniqSecret) error

	// Check checks if uniq secret exists in db by token.
	Check(ctx context.Context, token uuid.UUID) (_ bool, err error)

	// Revoke removes token from db.
	Revoke(ctx context.Context, token uuid.UUID) error
}

// Token stores secret of sno token.
type Token [32]byte

// UniqSecret describing secret model in the database.
type UniqSecret struct {
	// Secret is PK of the table and keeps unique value sno secret token
	Secret Token

	CreatedAt time.Time `json:"createdAt"`
}

// NewSecretToken creates new secret token.
func NewSecretToken() (Token, error) {
	var b [32]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return b, errs.New("error creating secret token")
	}

	return b, nil
}

// String implements Stringer.
func (secret Token) String() string {
	return base64.URLEncoding.EncodeToString(secret[:])
}

// IsZero returns if the secret token is not set.
func (secret Token) IsZero() bool {
	var zero Token
	// this doesn't need to be constant-time, because we're explicitly testing
	// against a hardcoded, well-known value
	return bytes.Equal(secret[:], zero[:])
}

// TokenSecretFromBase64 creates new secret token from base64 string.
func TokenSecretFromBase64(s string) (Token, error) {
	var secret Token

	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return secret, err
	}

	copy(secret[:], b)

	return secret, nil
}
