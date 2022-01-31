// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// ResetPasswordTokens is interface for working with reset password tokens.
//
// architecture: Database
type ResetPasswordTokens interface {
	// Create creates new reset password token
	Create(ctx context.Context, ownerID uuid.UUID) (*ResetPasswordToken, error)
	// GetBySecret retrieves ResetPasswordToken with given secret
	GetBySecret(ctx context.Context, secret ResetPasswordSecret) (*ResetPasswordToken, error)
	// GetByOwnerID retrieves ResetPasswordToken by ownerID
	GetByOwnerID(ctx context.Context, ownerID uuid.UUID) (*ResetPasswordToken, error)
	// Delete deletes ResetPasswordToken by ResetPasswordSecret
	Delete(ctx context.Context, secret ResetPasswordSecret) error
}

// ResetPasswordSecret stores secret of registration token.
type ResetPasswordSecret [32]byte

// ResetPasswordToken describing reset password model in the database.
type ResetPasswordToken struct {
	// Secret is PK of the table and keeps unique value for reset password token
	Secret ResetPasswordSecret
	// OwnerID stores current token owner ID
	OwnerID *uuid.UUID

	CreatedAt time.Time `json:"createdAt"`
}

// NewResetPasswordSecret creates new reset password secret.
func NewResetPasswordSecret() (ResetPasswordSecret, error) {
	var b [32]byte

	_, err := rand.Read(b[:])
	if err != nil {
		return b, errs.New("error creating registration secret")
	}

	return b, nil
}

// String implements Stringer.
func (secret ResetPasswordSecret) String() string {
	return base64.URLEncoding.EncodeToString(secret[:])
}

// ResetPasswordSecretFromBase64 creates new reset password secret from base64 string.
func ResetPasswordSecretFromBase64(s string) (ResetPasswordSecret, error) {
	var secret ResetPasswordSecret

	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return secret, err
	}

	copy(secret[:], b)

	return secret, nil
}
