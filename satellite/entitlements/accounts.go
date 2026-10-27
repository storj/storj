// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// AccountScopePrefix is the prefix used for account scopes in the database.
const AccountScopePrefix = "account_id:"

// Collection methods Stripe accepts for an invoice. An invoice with
// CollectionMethodSendInvoice is paid by the customer rather than charged to a saved
// payment method, and carries a due date.
const (
	CollectionMethodChargeAutomatically = "charge_automatically"
	CollectionMethodSendInvoice         = "send_invoice"
)

// ErrInvalidInvoicePayment is returned when an invoice payment configuration is invalid,
// such as a due period on an automatically charged invoice. It is deliberately not wrapped
// in Error so callers can tell an operator mistake apart from a failure of the service.
var ErrInvalidInvoicePayment = errs.Class("invalid invoice payment configuration")

// AccountFeatures defines the account-level features available to a user.
type AccountFeatures struct {
	// InvoicePaymentMethods is the exact set of Stripe payment method types the account's
	// invoices may be paid with. Empty means the satellite sets no restriction and Stripe
	// chooses, which is what accounts without an entitlement get.
	InvoicePaymentMethods []string `json:"invoice_payment_methods,omitempty"`
	// InvoiceCollectionMethod is charge_automatically or send_invoice. Empty leaves
	// Stripe's default of charging the customer's saved payment method.
	InvoiceCollectionMethod string `json:"invoice_collection_method,omitempty"`
	// InvoiceDaysUntilDue is how long the customer has to pay a send_invoice invoice;
	// 0 makes the invoice due as soon as it is created. Stripe requires it for
	// send_invoice and rejects it for anything else.
	InvoiceDaysUntilDue int `json:"invoice_days_until_due,omitempty"`
}

// IsZero reports whether the features carry no invoice payment configuration at all.
func (af AccountFeatures) IsZero() bool {
	return len(af.InvoicePaymentMethods) == 0 &&
		af.InvoiceCollectionMethod == "" &&
		af.InvoiceDaysUntilDue == 0
}

// Accounts separates account-related entitlements functionality.
type Accounts struct {
	service *Service
}

// Get retrieves the account features of a user by their user ID. A user with no
// entitlement is not an error; it yields the zero value, because the invoicing code reads
// this for every customer and almost none of them have a row.
func (a *Accounts) Get(ctx context.Context, userID uuid.UUID) (feats AccountFeatures, err error) {
	defer mon.Task()(&ctx)(&err)

	ent, err := a.service.db.GetByScope(ctx, ConvertUserIDToAccountScope(userID))
	if err != nil {
		if ErrNotFound.Has(err) {
			return AccountFeatures{}, nil
		}
		return AccountFeatures{}, Error.Wrap(err)
	}

	err = json.Unmarshal(ent.Features, &feats)
	return feats, Error.Wrap(err)
}

// SetInvoicePayment replaces the invoice payment configuration of an account. Passing the
// zero value clears it, restoring Stripe's own choice of payment methods. Validation lives
// here rather than in the admin handler so that every writer is covered.
func (a *Accounts) SetInvoicePayment(ctx context.Context, userID uuid.UUID, feats AccountFeatures) (err error) {
	defer mon.Task()(&ctx)(&err)

	normalized, err := normalizeInvoicePayment(feats)
	if err != nil {
		return err
	}

	scope := ConvertUserIDToAccountScope(userID)

	// Load current record (may not exist yet).
	ent, err := getEntitlementBeforeSet(ctx, a.service.db, scope)
	if err != nil {
		return Error.Wrap(err)
	}

	var features AccountFeatures
	if len(ent.Features) > 0 {
		if err = json.Unmarshal(ent.Features, &features); err != nil {
			return Error.Wrap(err)
		}
	}
	features.InvoicePaymentMethods = normalized.InvoicePaymentMethods
	features.InvoiceCollectionMethod = normalized.InvoiceCollectionMethod
	features.InvoiceDaysUntilDue = normalized.InvoiceDaysUntilDue

	return Error.Wrap(upsertNewEntitlement(ctx, a.service.db, ent, features))
}

// normalizeInvoicePayment lowercases and de-duplicates the payment methods and checks the
// collection method against the due period. The method types themselves are not checked
// against a list: Stripe rejects anything it does not know or that is not enabled for the
// account, and it is the only place that knows which those are.
func normalizeInvoicePayment(feats AccountFeatures) (AccountFeatures, error) {
	out := AccountFeatures{
		InvoiceCollectionMethod: normalizeMethod(feats.InvoiceCollectionMethod),
		InvoiceDaysUntilDue:     feats.InvoiceDaysUntilDue,
	}

	switch out.InvoiceCollectionMethod {
	case "", CollectionMethodChargeAutomatically:
		if out.InvoiceDaysUntilDue != 0 {
			return AccountFeatures{}, ErrInvalidInvoicePayment.New("days until due is only valid with %q", CollectionMethodSendInvoice)
		}
	case CollectionMethodSendInvoice:
		if out.InvoiceDaysUntilDue < 0 {
			return AccountFeatures{}, ErrInvalidInvoicePayment.New("%q requires a non-negative days until due", CollectionMethodSendInvoice)
		}
	default:
		return AccountFeatures{}, ErrInvalidInvoicePayment.New("unknown collection method %q", feats.InvoiceCollectionMethod)
	}

	for _, m := range feats.InvoicePaymentMethods {
		m = normalizeMethod(m)
		if m == "" {
			return AccountFeatures{}, ErrInvalidInvoicePayment.New("payment method cannot be empty")
		}
		if !slices.Contains(out.InvoicePaymentMethods, m) {
			out.InvoicePaymentMethods = append(out.InvoicePaymentMethods, m)
		}
	}

	return out, nil
}

func normalizeMethod(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// DeleteByUserID removes the account entitlements of a user.
func (a *Accounts) DeleteByUserID(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(a.service.db.DeleteByScope(ctx, ConvertUserIDToAccountScope(userID)))
}

// ConvertUserIDToAccountScope converts a user ID to a database account scope.
func ConvertUserIDToAccountScope(userID uuid.UUID) []byte {
	return append([]byte(AccountScopePrefix), userID[:]...)
}
