// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package secret

import (
	"context"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// ErrSecretService defines secret service error.
var ErrSecretService = errs.Class("secret service error")

// Service responsible for operations with storagenode's uniq secret.
//
// architecture: Service
type Service struct {
	store DB
}

// Issue generates new storagenode uniq secret and stores it into db.
func (service *Service) Issue(ctx context.Context) error {
	var secret UniqSecret

	token, err := NewSecretToken()
	if err != nil {
		return ErrSecretService.Wrap(err)
	}

	secret.Secret = token
	secret.CreatedAt = time.Now().UTC()

	err = service.store.Store(ctx, secret)
	if err != nil {
		return ErrSecretService.Wrap(err)
	}

	return nil
}

// Check returns boolean values if unique secret exists in db by secret token.
func (service *Service) Check(ctx context.Context, token uuid.UUID) (_ bool, err error) {
	isExists, err := service.store.Check(ctx, token)
	if err != nil {
		if ErrNoSecret.Has(err) {
			return false, nil
		}
		return false, ErrSecretService.Wrap(err)
	}

	return isExists, nil
}

// Remove revokes token, delete's it from db.
func (service *Service) Remove(ctx context.Context, token uuid.UUID) error {
	err := service.store.Revoke(ctx, token)
	if err != nil {
		return ErrSecretService.Wrap(err)
	}

	return nil
}
