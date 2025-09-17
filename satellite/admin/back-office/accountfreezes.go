// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/console"
)

// FreezeUserRequest represents a request to freeze a user account.
type FreezeUserRequest struct {
	Type console.AccountFreezeEventType `json:"type"`
}

// FreezeEventType represents a type of freeze event.
type FreezeEventType struct {
	Name  string                         `json:"name"`
	Value console.AccountFreezeEventType `json:"value"`
}

var freezeEventTypes = []FreezeEventType{
	{Name: console.BillingFreeze.String(), Value: console.BillingFreeze},
	{Name: console.LegalFreeze.String(), Value: console.LegalFreeze},
	{Name: console.ViolationFreeze.String(), Value: console.ViolationFreeze},
	{Name: console.TrialExpirationFreeze.String(), Value: console.TrialExpirationFreeze},
}

// GetFreezeEventTypes returns the available account freeze event types.
func (s *Service) GetFreezeEventTypes(ctx context.Context) ([]FreezeEventType, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	return freezeEventTypes, api.HTTPError{}
}

// FreezeUser freezes a user account by email address and freeze type.
func (s *Service) FreezeUser(ctx context.Context, userID uuid.UUID, request FreezeUserRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	switch request.Type {
	case console.LegalFreeze:
		err = s.accountFreeze.LegalFreezeUser(ctx, userID)
	case console.ViolationFreeze:
		err = s.accountFreeze.ViolationFreezeUser(ctx, userID)
	case console.TrialExpirationFreeze:
		err = s.accountFreeze.AdminTrialExpirationFreezeUser(ctx, userID)
	case console.BillingFreeze:
		err = s.accountFreeze.AdminBillingFreezeUser(ctx, userID)
	default:
		return api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("unsupported freeze event type %d", request.Type),
		}
	}
	if err != nil {
		status := http.StatusInternalServerError
		e := err
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			e = errors.New("user not found")
		}
		return api.HTTPError{
			Status: status,
			Err:    e,
		}
	}

	return api.HTTPError{}
}

// UnfreezeUser unfreezes a user account by user ID and freeze type.
func (s *Service) UnfreezeUser(ctx context.Context, userID uuid.UUID) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	_, err = s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	freezes, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusConflict
			err = errors.New("user is not frozen")
		}
		return api.HTTPError{
			Status: status,
			Err:    err,
		}
	}

	if freezes.BillingFreeze != nil {
		err = s.accountFreeze.AdminBillingUnfreezeUser(ctx, userID)
	} else if freezes.LegalFreeze != nil {
		err = s.accountFreeze.LegalUnfreezeUser(ctx, userID)
	} else if freezes.ViolationFreeze != nil {
		err = s.accountFreeze.ViolationUnfreezeUser(ctx, userID)
	} else if freezes.TrialExpirationFreeze != nil {
		err = s.accountFreeze.AdminTrialExpirationUnfreezeUser(ctx, userID)
	} else if freezes.BillingWarning != nil {
		err = s.accountFreeze.AdminBillingUnWarnUser(ctx, userID)
	} else if freezes.BotFreeze != nil {
		err = s.accountFreeze.BotUnfreezeUser(ctx, userID)
	} else {
		// remaining possible freeze event is delayed bot freeze, which technically
		// isn't a freeze so do nothing
	}
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	return api.HTTPError{}
}
