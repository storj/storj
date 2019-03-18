// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
)

// RegistrationTokens is interface for working with registration tokens
// TODO: remove after vanguard release
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

// RegistrationSecret stores secret of registration token
type RegistrationSecret [32]byte

// RegistrationToken describing api key model in the database
type RegistrationToken struct {
	// Secret is PK of the table and keeps unique value forRegToken
	Secret RegistrationSecret
	// OwnerID stores current token owner ID
	OwnerID *uuid.UUID

	// ProjectLimit defines how many projects user is able to create
	ProjectLimit int `json:"projectLimit"`

	CreatedAt time.Time `json:"createdAt"`
}

// NewRegistrationSecret creates new registration secret
func NewRegistrationSecret() (RegistrationSecret, error) {
	var b [32]byte

	n, err := io.ReadFull(rand.Reader, b[:])
	if err != nil || n != 32 {
		return b, errs.New("error creating registration secret")
	}

	return b, nil
}

// String implements Stringer
func (secret RegistrationSecret) String() string {
	return base64.URLEncoding.EncodeToString(secret[:])
}

// RegistrationSecretFromBase64 creates new registration secret from base64 string
func RegistrationSecretFromBase64(s string) (RegistrationSecret, error) {
	var secret RegistrationSecret

	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return secret, err
	}

	copy(secret[:], b)

	return secret, nil
}
