// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package apikeys

import (
	"context"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/private/multinodeauth"
)

var (
	// ErrService defines secret service error.
	ErrService = errs.Class("secret service")

	mon = monkit.Package()
)

// Service responsible for operations with storagenode's uniq secret.
//
// architecture: Service
type Service struct {
	store DB
}

// NewService is a constructor for service.
func NewService(db DB) *Service {
	return &Service{store: db}
}

// Issue generates new api key and stores it into db.
func (service *Service) Issue(ctx context.Context) (apiKey APIKey, err error) {
	defer mon.Task()(&ctx)(&err)
	secret, err := multinodeauth.NewSecret()
	if err != nil {
		return APIKey{}, ErrService.Wrap(err)
	}

	apiKey.Secret = secret
	apiKey.CreatedAt = time.Now().UTC()

	err = service.store.Store(ctx, apiKey)
	if err != nil {
		return APIKey{}, ErrService.Wrap(err)
	}

	return apiKey, nil
}

// Check returns error if api key does not exists.
func (service *Service) Check(ctx context.Context, secret multinodeauth.Secret) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.store.Check(ctx, secret)
}

// Remove revokes apikey, deletes it from db.
func (service *Service) Remove(ctx context.Context, secret multinodeauth.Secret) (err error) {
	defer mon.Task()(&ctx)(&err)

	return ErrService.Wrap(service.store.Revoke(ctx, secret))
}
