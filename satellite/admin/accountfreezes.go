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
	"storj.io/storj/satellite/admin/auditlogger"
	"storj.io/storj/satellite/admin/changehistory"
	"storj.io/storj/satellite/console"
)

const (
	// FreezeActionFreeze is the action to freeze a user account.
	FreezeActionFreeze = "freeze"
	// FreezeActionUnfreeze is the action to unfreeze a user account.
	FreezeActionUnfreeze = "unfreeze"
)

// ToggleInactivityExemptionRequest is the body for the toggle-inactivity-exemption endpoint.
type ToggleInactivityExemptionRequest struct {
	Exempt bool   `json:"exempt"`
	Reason string `json:"reason"`
}

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
	{Name: console.OptOutFreeze.String(), Value: console.OptOutFreeze},
	{Name: console.InactivityFreeze.String(), Value: console.InactivityFreeze},
}

// GetFreezeEventTypes returns the available account freeze event types.
func (s *Service) GetFreezeEventTypes(ctx context.Context) ([]FreezeEventType, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if s.tenantID == nil {
		return freezeEventTypes, api.HTTPError{}
	}

	// tenant admins cannot opt out freeze
	events := make([]FreezeEventType, 0, len(freezeEventTypes)-1)
	for _, eventType := range freezeEventTypes {
		if eventType.Value == console.OptOutFreeze {
			continue
		}
		events = append(events, eventType)
	}

	return events, api.HTTPError{}
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

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if s.adminConfig.HideFreezeActions {
		return apiError(http.StatusForbidden, errs.New("freeze actions are disabled"))
	}

	if s.tenantID != nil && request.Type == console.OptOutFreeze {
		return apiError(http.StatusForbidden, errs.New("opt out freeze action is disabled"))
	}

	hasPerm := func(perm Permission) bool {
		return s.authorizer.HasPermissions(authInfo, perm)
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, Error.New("reason is required"))
	}

	if request.Action == FreezeActionFreeze && !hasPerm(PermAccountSuspend) {
		return apiError(http.StatusForbidden, errs.New("not authorized to freeze accounts"))
	} else if request.Action == FreezeActionUnfreeze && !hasPerm(PermAccountReActivate) {
		return apiError(http.StatusForbidden, errs.New("not authorized to unfreeze accounts"))
	} else if request.Action != FreezeActionFreeze && request.Action != FreezeActionUnfreeze {
		return apiError(http.StatusBadRequest, Error.New("invalid action %q", request.Action))
	}

	if request.Action == FreezeActionFreeze {
		switch request.Type {
		case console.LegalFreeze, console.ViolationFreeze, console.TrialExpirationFreeze, console.BillingFreeze, console.OptOutFreeze, console.InactivityFreeze:
		default:
			return apiError(http.StatusBadRequest, Error.New("unsupported freeze event type %d", request.Type))
		}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return apiError(status, err)
	}
	if !s.userMatchesTenant(user.TenantID) {
		return apiError(http.StatusNotFound, errors.New("user not found"))
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
	case console.OptOutFreeze:
		err = s.accountFreeze.AdminOptOutFreezeUser(ctx, userID)
	case console.InactivityFreeze:
		err = s.accountFreeze.InactivityFreezeUser(ctx, userID)
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
		s.log.Error("failed to get account freeze state after change", zap.String("user_id", userID.String()), zap.Error(err))
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
	} else if freezes.OptOutFreeze != nil {
		err = s.accountFreeze.AdminOptOutUnfreezeUser(ctx, userID)
	} else if freezes.InactivityFreeze != nil {
		err = s.accountFreeze.InactivityUnfreezeUser(ctx, userID)
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
		s.log.Error("failed to get account freeze state after change", zap.String("user_id", userID.String()), zap.Error(err))
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

// ToggleInactivityExemption sets or clears the inactivity exemption flag for a user.
// When granting (Exempt=true) it also clears any existing InactivityWarning or InactivityFreeze events.
func (s *Service) ToggleInactivityExemption(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request ToggleInactivityExemptionRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{Status: status, Err: Error.Wrap(err)}
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}
	if !s.authorizer.HasPermissions(authInfo, PermManageInactivityExemption) {
		return apiError(http.StatusForbidden, errs.New("not authorized"))
	}
	if request.Reason == "" {
		return apiError(http.StatusBadRequest, Error.New("reason is required"))
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return apiError(status, err)
	}
	if !s.userMatchesTenant(user.TenantID) {
		return apiError(http.StatusNotFound, errors.New("user not found"))
	}

	beforeState, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	if request.Exempt {
		if beforeState != nil && beforeState.InactivityFreeze != nil {
			if err = s.accountFreeze.InactivityUnfreezeUser(ctx, userID); err != nil {
				return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
			}
		} else if beforeState != nil && beforeState.InactivityWarning != nil {
			if err = s.accountFreeze.InactivityUnwarnUser(ctx, userID); err != nil {
				return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
			}
		}
	}

	if err = s.consoleDB.Users().UpsertSettings(ctx, userID, console.UpsertUserSettingsRequest{
		InactivityExempt: &request.Exempt,
	}); err != nil {
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	afterState, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.log.Error("failed to get freeze state after exemption toggle", zap.String("user_id", userID.String()), zap.Error(err))
	} else {
		action := "revoke_inactivity_exemption"
		if request.Exempt {
			action = "grant_inactivity_exemption"
		}
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     userID,
			Action:     action,
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
