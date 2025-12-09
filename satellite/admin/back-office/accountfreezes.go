// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/console"
)

const (
	// FreezeActionFreeze is the action to freeze a user account.
	FreezeActionFreeze = "freeze"
	// FreezeActionUnfreeze is the action to unfreeze a user account.
	FreezeActionUnfreeze = "unfreeze"
)

// ToggleFreezeUserRequest represents a request to freeze a user account.
type ToggleFreezeUserRequest struct {
	Action string                         `json:"action"` // should be "freeze" or "unfreeze"
	Type   console.AccountFreezeEventType `json:"type"`

	Reason string `json:"reason"`
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

// ToggleFreezeUser freezes or unfreezes a user account by email address.
func (s *Service) ToggleFreezeUser(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request ToggleFreezeUserRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if authInfo == nil || len(authInfo.Groups) == 0 {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	hasPerm := func(perm Permission) bool {
		for _, g := range authInfo.Groups {
			if s.authorizer.HasPermissions(g, perm) {
				return true
			}
		}
		return false
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, Error.New("reason is required"))
	}

	if request.Action == FreezeActionFreeze && !hasPerm(PermAccountSuspendTemporary) {
		return apiError(http.StatusForbidden, errs.New("not authorized to freeze accounts"))
	} else if request.Action == FreezeActionUnfreeze && !hasPerm(PermAccountReActivateTemporary) {
		return apiError(http.StatusForbidden, errs.New("not authorized to unfreeze accounts"))
	} else if request.Action != FreezeActionFreeze && request.Action != FreezeActionUnfreeze {
		return apiError(http.StatusBadRequest, Error.New("invalid action %q", request.Action))
	}

	beforeState, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	if request.Action == FreezeActionUnfreeze {
		return s.unfreezeUser(ctx, authInfo, userID, request.Reason)
	}

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

	afterState, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.log.Error("failed to get account freeze state after change", zap.String("userID", userID.String()), zap.Error(err))
	} else {
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     userID,
			Action:     "toggle_freeze_user",
			AdminEmail: authInfo.Email,
			ItemType:   changehistory.ItemTypeUser,
			Reason:     request.Reason,
			Before:     beforeState,
			After:      afterState,
			Timestamp:  s.nowFn(),
		})
	}

	return api.HTTPError{}
}

// unfreezeUser unfreezes a user account by user ID and freeze type.
func (s *Service) unfreezeUser(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, reason string) api.HTTPError {
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

	afterState, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.log.Error("failed to get account freeze state after change", zap.String("userID", userID.String()), zap.Error(err))
	} else {
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     userID,
			Action:     "unfreeze_user",
			AdminEmail: authInfo.Email,
			ItemType:   changehistory.ItemTypeUser,
			Reason:     reason,
			Before:     freezes,
			After:      afterState,
			Timestamp:  s.nowFn(),
		})
	}

	return api.HTTPError{}
}
