// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// APIKeyTails is interface for working with api key tails store.
//
// architecture: Database
type APIKeyTails interface {
	// Upsert is a method for inserting or updating APIKeyTail in the database.
	Upsert(ctx context.Context, tail *APIKeyTail) (*APIKeyTail, error)
	// UpsertBatch is a method for inserting or updating a batch of APIKeyTails in the database.
	UpsertBatch(ctx context.Context, batch []APIKeyTail) error
	// GetByTail retrieves APIKeyTail for given key tail.
	GetByTail(ctx context.Context, tail []byte) (*APIKeyTail, error)
	// CheckExistenceBatch checks the existence of multiple tails in a single query.
	// Returns a map where key is hex-encoded tail and value indicates existence.
	CheckExistenceBatch(ctx context.Context, tails [][]byte) (map[string]bool, error)
}

// APIKeyTail describing api key tail model in the database.
type APIKeyTail struct {
	RootKeyID  uuid.UUID `json:"rootKeyID"`
	Tail       []byte    `json:"tail"`
	ParentTail []byte    `json:"parentTail"`
	Caveat     []byte    `json:"caveat"`
	LastUsed   time.Time `json:"lastUsed"`
}
