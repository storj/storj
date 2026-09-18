// Copyright (C) 2026 Storj Labs, Inc.
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
	"storj.io/storj/satellite/entitlements"
)

// UserEntitlements represents the account-level entitlements of a user.
type UserEntitlements struct {
	InvoicePaymentMethods   []string `json:"invoicePaymentMethods"`
	InvoiceCollectionMethod string   `json:"invoiceCollectionMethod"`
	InvoiceDaysUntilDue     int      `json:"invoiceDaysUntilDue"`
}

// SetInvoicePaymentRequest represents a request to replace the payment configuration of a
// user's invoices. The three fields are only meaningful together, so they are set as a set.
type SetInvoicePaymentRequest struct {
	PaymentMethods   []string `json:"paymentMethods"`
	CollectionMethod string   `json:"collectionMethod"`
	DaysUntilDue     int      `json:"daysUntilDue"`
	Reason           string   `json:"reason"`
}

// GetUserEntitlements returns the account-level entitlements of a user by their ID.
func (s *Service) GetUserEntitlements(ctx context.Context, userID uuid.UUID) (*UserEntitlements, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if s.tenantID != nil {
		return nil, apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apiError(http.StatusNotFound, errs.New("user not found"))
		}
		return nil, apiError(http.StatusInternalServerError, err)
	}

	feats, err := s.entitlements.Accounts().Get(ctx, user.ID)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}

	return toUserEntitlements(feats), api.HTTPError{}
}

// SetUserInvoicePayment replaces the payment configuration of a user's invoices. An empty
// request clears the entitlement and restores Stripe's automatic selection.
func (s *Service) SetUserInvoicePayment(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request SetInvoicePaymentRequest) (*UserEntitlements, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if s.tenantID != nil {
		return nil, apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return nil, apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return nil, apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, apiError(http.StatusNotFound, errs.New("user not found"))
		}
		return nil, apiError(http.StatusInternalServerError, err)
	}

	// Get returns a freshly unmarshalled value that nothing else holds, so the audit
	// logger can keep it as the before state without copying.
	beforeState, err := s.entitlements.Accounts().Get(ctx, user.ID)
	if err != nil {
		return nil, apiError(http.StatusInternalServerError, err)
	}

	err = s.entitlements.Accounts().SetInvoicePayment(ctx, user.ID, entitlements.AccountFeatures{
		InvoicePaymentMethods:   request.PaymentMethods,
		InvoiceCollectionMethod: request.CollectionMethod,
		InvoiceDaysUntilDue:     request.DaysUntilDue,
	})
	if err != nil {
		// An invalid configuration is an operator mistake, not a failure of the service.
		if entitlements.ErrInvalidInvoicePayment.Has(err) {
			return nil, apiError(http.StatusBadRequest, err)
		}
		return nil, apiError(http.StatusInternalServerError, err)
	}

	afterState, err := s.entitlements.Accounts().Get(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to retrieve entitlements after setting invoice payment",
			zap.Stringer("user_id", user.ID), zap.Error(err))
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "set_user_invoice_payment",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     beforeState,
		After:      afterState,
		Timestamp:  s.nowFn(),
	})

	return toUserEntitlements(afterState), api.HTTPError{}
}

func toUserEntitlements(feats entitlements.AccountFeatures) *UserEntitlements {
	// A nil slice would render as null; the API always reports a list.
	methods := feats.InvoicePaymentMethods
	if methods == nil {
		methods = []string{}
	}

	return &UserEntitlements{
		InvoicePaymentMethods:   methods,
		InvoiceCollectionMethod: feats.InvoiceCollectionMethod,
		InvoiceDaysUntilDue:     feats.InvoiceDaysUntilDue,
	}
}
