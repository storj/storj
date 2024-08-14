// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// ErrService is the default error class for the authorization service.
var ErrService = errs.Class("authorization service")

// Service is the authorization service.
type Service struct {
	log *zap.Logger
	db  *DB
}

// NewService creates a new authorization service.
func NewService(log *zap.Logger, db *DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// GetOrCreate will return an authorization for the given user ID.
func (service *Service) GetOrCreate(ctx context.Context, userID string) (_ *Token, err error) {
	defer mon.Task()(&ctx)(&err)

	if userID == "" {
		msg := "missing user ID"
		err = ErrService.New("%v", msg)
		return nil, err
	}

	existingGroup, err := service.db.Get(ctx, userID)
	if err != nil && !ErrNotFound.Has(err) {
		msg := "error getting authorizations"
		err = ErrService.Wrap(err)
		service.log.Error(msg, zap.Error(err))
		return nil, err
	}

	for _, authorization := range existingGroup {
		if authorization.Claim == nil {
			return &authorization.Token, nil
		}
	}

	createdGroup, err := service.db.Create(ctx, userID, 1)
	if err != nil {
		msg := "error creating authorization"
		err = ErrService.Wrap(err)
		service.log.Error(msg, zap.Error(err))
		return nil, err
	}

	groupLen := len(createdGroup)
	if groupLen != 1 {
		clientMsg := "error creating authorization"
		internalMsg := clientMsg + fmt.Sprintf("; expected 1, got %d", groupLen)

		service.log.Error(internalMsg)
		return nil, ErrEndpoint.New("%s", clientMsg)
	}

	authorization := createdGroup[0]
	return &authorization.Token, nil
}
