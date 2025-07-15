// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleservice

import (
	"context"
	"net/http"
	"time"

	"storj.io/storj/private/api"
	"storj.io/storj/satellite/console"
)

// Users separates all users related functionality.
type Users struct {
	service *Service
}

// GetUserAccount returns account info for authenticated user.
func (u Users) GetUserAccount(ctx context.Context, authUser *console.User) (*console.UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	u.service.auditLog(ctx, "get user account", &authUser.ID, authUser.Email)

	freezes, err := u.service.deps.AccountFreezeService.GetAll(ctx, authUser.ID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	userAccount := &console.UserAccount{
		FreezeStatus: console.FreezeStat{
			Frozen:             freezes.BillingFreeze != nil,
			Warned:             freezes.BillingWarning != nil,
			TrialExpiredFrozen: freezes.TrialExpirationFreeze != nil,
		},
	}
	if userAccount.FreezeStatus.TrialExpiredFrozen {
		days := u.service.deps.AccountFreezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, time.Now())
		if days != nil && *days > 0 {
			userAccount.FreezeStatus.TrialExpirationGracePeriod = *days
		}
	}

	userAccount.ShortName = authUser.ShortName
	userAccount.FullName = authUser.FullName
	userAccount.Email = authUser.Email
	userAccount.ID = authUser.ID
	if authUser.ExternalID != nil {
		userAccount.ExternalID = *authUser.ExternalID
	}
	if authUser.UserAgent != nil {
		userAccount.Partner = string(authUser.UserAgent)
	}
	userAccount.ProjectLimit = authUser.ProjectLimit
	userAccount.ProjectStorageLimit = authUser.ProjectStorageLimit
	userAccount.ProjectBandwidthLimit = authUser.ProjectBandwidthLimit
	userAccount.ProjectSegmentLimit = authUser.ProjectSegmentLimit
	userAccount.IsProfessional = authUser.IsProfessional
	userAccount.CompanyName = authUser.CompanyName
	userAccount.Position = authUser.Position
	userAccount.EmployeeCount = authUser.EmployeeCount
	userAccount.HaveSalesContact = authUser.HaveSalesContact
	userAccount.PaidTier = authUser.IsPaid()
	userAccount.Kind = authUser.Kind.Info()
	userAccount.MFAEnabled = authUser.MFAEnabled
	userAccount.MFARecoveryCodeCount = len(authUser.MFARecoveryCodes)
	userAccount.CreatedAt = authUser.CreatedAt
	userAccount.PendingVerification = authUser.Status == console.PendingBotVerification
	userAccount.TrialExpiration = authUser.TrialExpiration
	userAccount.HasVarPartner = u.GetUserHasVarPartner(ctx, authUser)

	return userAccount, api.HTTPError{}
}

// GetUserHasVarPartner returns whether the user in context is associated with a VAR partner.
func (u Users) GetUserHasVarPartner(ctx context.Context, authUser *console.User) (has bool) {
	var err error
	defer mon.Task()(&ctx)(&err)

	u.service.auditLog(ctx, "get user has VAR partner", &authUser.ID, authUser.Email)

	_, has = u.service.internal.varPartners[string(authUser.UserAgent)]
	return has
}
