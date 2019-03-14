// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// RegTokens is interface for working with registration tokens
// TODO: remove after vanguard release
type RegTokens interface {
	// CreateRegToken creates new registration token
	CreateRegToken(ctx context.Context, projLimit int) (*RegToken, error)
	// GetBySecret retrieves RegTokenInfo with given Secret
	GetBySecret(ctx context.Context, secret uuid.UUID) (*RegToken, error)
	// GetByOwnerID retrieves RegTokenInfo by ownerID
	GetByOwnerID(ctx context.Context, ownerID uuid.UUID) (*RegToken, error)
	// UpdateOwner updates registration token's owner
	UpdateOwner(ctx context.Context, secret, ownerID uuid.UUID) error
}

// RegToken describing api key model in the database
type RegToken struct {
	// Secret is PK of the table and keeps unique value forRegToken
	Secret uuid.UUID
	// OwnerID stores current token owner ID
	OwnerID *uuid.UUID

	// ProjLimit defines how many projects user is able to create
	ProjLimit int `json:"projLimit"`

	CreatedAt time.Time `json:"createdAt"`
}
