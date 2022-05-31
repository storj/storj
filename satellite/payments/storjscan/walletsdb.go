// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
)

// WalletsDB is an interface which defines functionality
// of DB which stores user storjscan wallets.
//
// architecture: Database
type WalletsDB interface {
	// Add adds a new storjscan wallet to the DB and associates it with a user.
	Add(ctx context.Context, userID uuid.UUID, walletAddress blockchain.Address) error
	// Get returns the wallet address associated with the given user.
	Get(ctx context.Context, userID uuid.UUID) (blockchain.Address, error)
	// GetAllUsers returns all user IDs that have associated storjscan wallets.
	GetAllUsers(ctx context.Context) (_ []uuid.UUID, err error)
}
