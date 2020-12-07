// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// RegistrationTokens is interface for working with registration tokens.
//
// architecture: Database
type RegistrationTokens interface {
	// Create creates new registration token
	Create(ctx context.Context, projectLimit int) (*RegistrationToken, error)
	// GetBySecret retrieves RegTokenInfo with given Secret
	GetBySecret(ctx context.Context, secret RegistrationSecret) (*RegistrationToken, error)
	// GetByOwnerID retrieves RegTokenInfo by ownerID
	GetByOwnerID(ctx context.Context, ownerID uuid.UUID) (*RegistrationToken, error)
	// UpdateOwner updates registration token's owner
	UpdateOwner(ctx context.Context, secret RegistrationSecret, ownerID uuid.UUID) error
}

// RegistrationSecret stores secret of registration token.
type RegistrationSecret [32]byte

// RegistrationToken describing api key model in the database.
type RegistrationToken struct {
	// Secret is PK of the table and keeps unique value forRegToken
	Secret RegistrationSecret
	// OwnerID stores current token owner ID
	OwnerID *uuid.UUID

	// ProjectLimit defines how many projects user is able to create
	ProjectLimit int `json:"projectLimit"`

	CreatedAt time.Time `json:"createdAt"`
}

// NewRegistrationSecret creates new registration secret.
func NewRegistrationSecret() (RegistrationSecret, error) {
	var b [32]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return b, errs.New("error creating registration secret")
	}

	return b, nil
}

// String implements Stringer.
func (secret RegistrationSecret) String() string {
	return base64.URLEncoding.EncodeToString(secret[:])
}

// IsZero returns if the RegistrationSecret is not set.
func (secret RegistrationSecret) IsZero() bool {
	var zero RegistrationSecret
	// this doesn't need to be constant-time, because we're explicitly testing
	// against a hardcoded, well-known value
	return bytes.Equal(secret[:], zero[:])
}

// RegistrationSecretFromBase64 creates new registration secret from base64 string.
func RegistrationSecretFromBase64(s string) (RegistrationSecret, error) {
	var secret RegistrationSecret

	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return secret, err
	}

	copy(secret[:], b)

	return secret, nil
}
