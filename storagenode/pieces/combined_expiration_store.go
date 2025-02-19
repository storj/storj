// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package pieces

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
)

// CombinedExpirationStore prefers the flat file store for setting expirations but
// uses both the flat file store and the chained database for getting and deleting expirations.
type CombinedExpirationStore struct {
	log           *zap.Logger
	chainedDB     PieceExpirationDB
	flatFileStore *PieceExpirationStore
}

// NewCombinedExpirationStore creates a new CombinedExpirationStore.
func NewCombinedExpirationStore(log *zap.Logger, chainedDB PieceExpirationDB, flatFileStore *PieceExpirationStore) *CombinedExpirationStore {
	return &CombinedExpirationStore{
		log:           log,
		chainedDB:     chainedDB,
		flatFileStore: flatFileStore,
	}
}

// SetExpiration sets the expiration for a piece.
func (c *CombinedExpirationStore) SetExpiration(ctx context.Context, satellite storj.NodeID, pieceID storj.PieceID, expiresAt time.Time, pieceSize int64) error {
	return c.flatFileStore.SetExpiration(ctx, satellite, pieceID, expiresAt, pieceSize)
}

// GetExpired returns the expired pieces.
func (c *CombinedExpirationStore) GetExpired(ctx context.Context, expiresBefore time.Time, opts ExpirationOptions) ([]*ExpiredInfoRecords, error) {
	var errList errs.Group
	expired, err := c.flatFileStore.GetExpired(ctx, expiresBefore, opts)
	if err != nil {
		errList.Add(err)
	}

	if c.chainedDB != nil {
		chainedExpired, err := c.chainedDB.GetExpired(ctx, expiresBefore, opts)
		if err != nil {
			errList.Add(err)
		}
		expired = append(expired, chainedExpired...)
	}

	return expired, errList.Err()
}

// DeleteExpirations deletes the expirations for the given time.
func (c *CombinedExpirationStore) DeleteExpirations(ctx context.Context, expiresAt time.Time) error {
	var errList errs.Group

	c.log.Debug("deleting expired pieces from flat file store", zap.Time("expiresAt", expiresAt))
	if err := c.flatFileStore.DeleteExpirations(ctx, expiresAt); err != nil {
		errList.Add(err)
	}

	if c.chainedDB != nil {
		c.log.Debug("deleting expired pieces from chained db", zap.Time("expiresAt", expiresAt))
		errList.Add(c.chainedDB.DeleteExpirations(ctx, expiresAt))
	}

	return errList.Err()
}

// DeleteExpirationsBatch deletes the expirations for the given time.
func (c *CombinedExpirationStore) DeleteExpirationsBatch(ctx context.Context, now time.Time, opts ExpirationOptions) error {
	var errList errs.Group

	c.log.Debug("deleting expired pieces from flat file store", zap.Time("expiresAt", now), zap.Any("opts", opts))
	if err := c.flatFileStore.DeleteExpirationsBatch(ctx, now, opts); err != nil {
		errList.Add(err)
	}

	if c.chainedDB != nil {
		c.log.Debug("deleting expired pieces from flat file store", zap.Time("expiresAt", now), zap.Any("opts", opts))
		errList.Add(c.chainedDB.DeleteExpirationsBatch(ctx, now, opts))
	}

	return errList.Err()
}

var _ PieceExpirationDB = (*CombinedExpirationStore)(nil)
