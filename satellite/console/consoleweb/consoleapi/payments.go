// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/payments/stripe"
)

var (
	// ErrPaymentsAPI - console payments api error type.
	ErrPaymentsAPI = errs.Class("consoleapi payments")
	mon            = monkit.Package()
)

// Payments is an api controller that exposes all payment related functionality.
type Payments struct {
	log                  *zap.Logger
	service              *console.Service
	accountFreezeService *console.AccountFreezeService
	packagePlans         paymentsconfig.PackagePlans
}

// NewPayments is a constructor for api payments controller.
func NewPayments(log *zap.Logger, service *console.Service, accountFreezeService *console.AccountFreezeService, packagePlans paymentsconfig.PackagePlans) *Payments {
	return &Payments{
		log:                  log,
		service:              service,
		accountFreezeService: accountFreezeService,
		packagePlans:         packagePlans,
	}
}

// SetupAccount creates a payment account for the user.
func (p *Payments) SetupAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	couponType, err := p.service.Payments().SetupAccount(ctx)

	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(couponType)
	if err != nil {
		p.log.Error("failed to write json token deposit response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// AccountBalance returns an integer amount in cents that represents the current balance of payment account.
func (p *Payments) AccountBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	balance, err := p.service.Payments().AccountBalance(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(&balance)
	if err != nil {
		p.log.Error("failed to write json balance response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// ProjectsCharges returns how much money current user will be charged for each project which he owns.
func (p *Payments) ProjectsCharges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var response struct {
		PriceModels map[string]payments.ProjectUsagePriceModel `json:"priceModels"`
		Charges     payments.ProjectChargesResponse            `json:"charges"`
	}

	w.Header().Set("Content-Type", "application/json")

	sinceStamp, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}
	beforeStamp, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	since := time.Unix(sinceStamp, 0).UTC()
	before := time.Unix(beforeStamp, 0).UTC()

	charges, err := p.service.Payments().ProjectsCharges(ctx, since, before)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	response.Charges = charges
	response.PriceModels = make(map[string]payments.ProjectUsagePriceModel)

	seen := make(map[string]struct{})
	for _, partnerCharges := range charges {
		for partner := range partnerCharges {
			if _, ok := seen[partner]; ok {
				continue
			}
			response.PriceModels[partner] = *p.service.Payments().GetProjectUsagePriceModel(partner)
			seen[partner] = struct{}{}
		}
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		p.log.Error("failed to write json project usage and charges response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// TriggerAttemptPayment attempts payment of overdue invoices and unfreezes/unwarn user if needed.
func (p *Payments) TriggerAttemptPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	userID, err := p.service.GetUserID(ctx)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	freezes, err := p.accountFreezeService.GetAll(ctx, userID)
	if err != nil {
		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}

	if freezes.ViolationFreeze != nil {
		return
	}

	if freezes.BillingFreeze == nil && freezes.BillingWarning == nil && freezes.TrialExpirationFreeze == nil {
		return
	}

	err = p.service.Payments().AttemptPayOverdueInvoices(ctx)
	if err != nil {
		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}

	if freezes.BillingFreeze != nil {
		err = p.accountFreezeService.BillingUnfreezeUser(ctx, userID)
		if err != nil {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, errors.New("failed to unfreeze account"))
			return
		}
	} else if freezes.BillingWarning != nil {
		err = p.accountFreezeService.BillingUnWarnUser(ctx, userID)
		if err != nil {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, errors.New("failed to unfreeze account"))
			return
		}
	}
}

// AddCreditCard is used to save new credit card and attach it to payment account.
func (p *Payments) AddCreditCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	token := string(bodyBytes)

	_, err = p.service.Payments().AddCreditCard(ctx, token)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			web.ServeCustomJSONError(ctx, p.log, w, http.StatusUnauthorized, err, errs.Unwrap(err).Error())
			return
		}

		if stripe.ErrDuplicateCard.Has(err) {
			web.ServeCustomJSONError(ctx, p.log, w, http.StatusBadRequest, err, errs.Unwrap(err).Error())
			return
		}

		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}
}

// AddCardByPaymentMethodID is used to save new credit card and attach it to payment account.
// It uses payment method id instead of token.
func (p *Payments) AddCardByPaymentMethodID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	pmID := string(bodyBytes)

	_, err = p.service.Payments().AddCardByPaymentMethodID(ctx, pmID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			web.ServeCustomJSONError(ctx, p.log, w, http.StatusUnauthorized, err, errs.Unwrap(err).Error())
			return
		}

		if stripe.ErrDuplicateCard.Has(err) {
			web.ServeCustomJSONError(ctx, p.log, w, http.StatusBadRequest, err, errs.Unwrap(err).Error())
			return
		}

		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}
}

// ListCreditCards returns a list of credit cards for a given payment account.
func (p *Payments) ListCreditCards(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	cards, err := p.service.Payments().ListCreditCards(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if cards == nil {
		_, err = w.Write([]byte("[]"))
	} else {
		err = json.NewEncoder(w).Encode(cards)
	}

	if err != nil {
		p.log.Error("failed to write json list cards response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// MakeCreditCardDefault makes a credit card default payment method.
func (p *Payments) MakeCreditCardDefault(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	cardID, err := io.ReadAll(r.Body)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	err = p.service.Payments().MakeCreditCardDefault(ctx, string(cardID))
	if err != nil {
		if stripe.ErrCardNotFound.Has(err) {
			p.serveJSONError(ctx, w, http.StatusNotFound, err)
			return
		}

		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// RemoveCreditCard is used to detach a credit card from payment account.
func (p *Payments) RemoveCreditCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	vars := mux.Vars(r)
	cardID := vars["cardId"]

	if cardID == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	err = p.service.Payments().RemoveCreditCard(ctx, cardID)
	if err != nil {
		if stripe.ErrCardNotFound.Has(err) {
			p.serveJSONError(ctx, w, http.StatusNotFound, err)
			return
		}
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// BillingHistory returns a list of invoices, transactions and all others billing history items for payment account.
func (p *Payments) BillingHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	billingHistory, err := p.service.Payments().BillingHistory(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if billingHistory == nil {
		_, err = w.Write([]byte("[]"))
	} else {
		err = json.NewEncoder(w).Encode(billingHistory)
	}

	if err != nil {
		p.log.Error("failed to write json billing history response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// InvoiceHistory returns a paged list of invoice history items for payment account.
func (p *Payments) InvoiceHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	query := r.URL.Query()

	limitParam := query.Get("limit")
	if limitParam == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'limit' is required"))
		return
	}

	limit, pErr := strconv.ParseUint(limitParam, 10, 32)
	if pErr != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	startParam := query.Get("starting_after")
	endParam := query.Get("ending_before")

	history, err := p.service.Payments().InvoiceHistory(ctx, payments.InvoiceCursor{
		Limit:         int(limit),
		StartingAfter: startParam,
		EndingBefore:  endParam,
	})
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(history)
	if err != nil {
		p.log.Error("failed to write json history response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// ApplyCouponCode applies a coupon code to the user's account.
func (p *Payments) ApplyCouponCode(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
	couponCode := string(bodyBytes)

	coupon, err := p.service.Payments().ApplyCouponCode(ctx, couponCode)
	if err != nil {
		status := http.StatusInternalServerError
		if payments.ErrInvalidCoupon.Has(err) {
			status = http.StatusBadRequest
		} else if payments.ErrCouponConflict.Has(err) {
			status = http.StatusConflict
		}
		p.serveJSONError(ctx, w, status, err)
		return
	}

	if err = json.NewEncoder(w).Encode(coupon); err != nil {
		p.log.Error("failed to encode coupon", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// GetCoupon returns the coupon applied to the user's account.
func (p *Payments) GetCoupon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	coupon, err := p.service.Payments().GetCoupon(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = json.NewEncoder(w).Encode(coupon); err != nil {
		p.log.Error("failed to encode coupon", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// GetWallet returns the wallet address (with balance) already assigned to the user.
func (p *Payments) GetWallet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	walletInfo, err := p.service.Payments().GetWallet(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if errs.Is(err, billing.ErrNoWallet) {
			p.serveJSONError(ctx, w, http.StatusNotFound, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = json.NewEncoder(w).Encode(walletInfo); err != nil {
		p.log.Error("failed to encode wallet info", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// ClaimWallet will claim a new wallet address. Returns with existing if it's already claimed.
func (p *Payments) ClaimWallet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	walletInfo, err := p.service.Payments().ClaimWallet(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = json.NewEncoder(w).Encode(walletInfo); err != nil {
		p.log.Error("failed to encode wallet info", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// WalletPayments returns with the list of storjscan transactions for user`s wallet.
func (p *Payments) WalletPayments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	var walletPayments console.WalletPayments
	walletPayments, err = p.service.Payments().WalletPayments(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if errs.Is(err, billing.ErrNoWallet) {
			if err = json.NewEncoder(w).Encode(walletPayments); err != nil {
				p.log.Error("failed to encode payments", zap.Error(ErrPaymentsAPI.Wrap(err)))
			}
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = json.NewEncoder(w).Encode(walletPayments); err != nil {
		p.log.Error("failed to encode payments", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// WalletPaymentsWithConfirmations returns with the list of storjscan transactions (including confirmations count) for user`s wallet.
func (p *Payments) WalletPaymentsWithConfirmations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	walletPayments, err := p.service.Payments().WalletPaymentsWithConfirmations(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if errs.Is(err, billing.ErrNoWallet) {
			if err = json.NewEncoder(w).Encode([]string{}); err != nil {
				p.log.Error("failed to encode payments", zap.Error(ErrPaymentsAPI.Wrap(err)))
			}
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = json.NewEncoder(w).Encode(walletPayments); err != nil {
		p.log.Error("failed to encode wallet payments with confirmations", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// GetProjectUsagePriceModel returns the project usage price model for the user.
func (p *Payments) GetProjectUsagePriceModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	user, err := console.GetUser(ctx)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	pricing := p.service.Payments().GetProjectUsagePriceModel(string(user.UserAgent))

	if err = json.NewEncoder(w).Encode(pricing); err != nil {
		p.log.Error("failed to encode project usage price model", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// PurchasePackage purchases one of the configured paymentsconfig.PackagePlans.
func (p *Payments) PurchasePackage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	// whether to use payment method id instead of token for adding card.
	usePmID := r.URL.Query().Get("pmID") == "true"

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	token := string(bodyBytes)

	u, err := console.GetUser(ctx)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	pkg, err := p.packagePlans.Get(u.UserAgent)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusNotFound, err)
		return
	}

	var addCardFunc func(context.Context, string) (payments.CreditCard, error)
	if usePmID {
		addCardFunc = p.service.Payments().AddCardByPaymentMethodID
	} else {
		addCardFunc = p.service.Payments().AddCreditCard
	}

	card, err := addCardFunc(ctx, token)
	if err != nil {
		switch {
		case console.ErrUnauthorized.Has(err):
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		default:
			p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		}
		return
	}

	description := fmt.Sprintf("%s package plan", string(u.UserAgent))
	err = p.service.Payments().UpdatePackage(ctx, description, time.Now())
	if err != nil {
		if !console.ErrAlreadyHasPackage.Has(err) {
			p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
			return
		}
	}

	err = p.service.Payments().Purchase(ctx, pkg.Price, description, card.ID)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = p.service.Payments().ApplyCredit(ctx, pkg.Credit, description); err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// PackageAvailable returns whether a package plan is configured for the user's partner.
func (p *Payments) PackageAvailable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	u, err := console.GetUser(ctx)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	pkg, err := p.packagePlans.Get(u.UserAgent)
	hasPkg := err == nil && pkg != payments.PackagePlan{}

	if err = json.NewEncoder(w).Encode(hasPkg); err != nil {
		p.log.Error("failed to encode package plan checking response", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// GetTaxCountries returns a list of countries whose taxes are supported.
func (p *Payments) GetTaxCountries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(payments.TaxCountries); err != nil {
		p.log.Error("failed to encode project usage price model", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// GetCountryTaxes returns a list of taxes supported for a country.
func (p *Payments) GetCountryTaxes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	var countryCodeStr string
	var ok bool
	if countryCodeStr, ok = mux.Vars(r)["countryCode"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("country code is required"))
		return

	}

	ts := make([]payments.Tax, 0)
	for _, tax := range payments.Taxes {
		if tax.CountryCode == payments.CountryCode(countryCodeStr) {
			ts = append(ts, tax)
		}
	}

	if err = json.NewEncoder(w).Encode(ts); err != nil {
		p.log.Error("failed to encode project usage price model", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// GetBillingInformation gets the billing information for a user.
func (p *Payments) GetBillingInformation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	information, err := p.service.Payments().GetBillingInformation(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}

	if err = json.NewEncoder(w).Encode(information); err != nil {
		p.log.Error("failed encode billing information", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// SaveBillingAddress saves billing address for a user.
func (p *Payments) SaveBillingAddress(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var address payments.BillingAddress
	if err = json.NewDecoder(r.Body).Decode(&address); err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if address.Name == "" || address.Line1 == "" ||
		address.City == "" || address.Country.Code == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing required address fields"))
		return
	}

	newInfo, err := p.service.Payments().SaveBillingAddress(ctx, address)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}

	if err = json.NewEncoder(w).Encode(newInfo); err != nil {
		p.log.Error("failed encode billing information", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// AddInvoiceReference adds a default invoice reference to be displayed on every invoice.
func (p *Payments) AddInvoiceReference(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var body struct {
		Reference string `json:"reference"`
	}
	if err = json.NewDecoder(r.Body).Decode(&body); err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if len(body.Reference) > 140 {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("reference is too long"))
		return
	}

	newInfo, err := p.service.Payments().AddInvoiceReference(ctx, body.Reference)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}

	if err = json.NewEncoder(w).Encode(newInfo); err != nil {
		p.log.Error("failed encode billing information", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// AddTaxID adds a tax ID to a user.
func (p *Payments) AddTaxID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var taxID payments.TaxID
	if err = json.NewDecoder(r.Body).Decode(&taxID); err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if taxID.Tax.Code == "" || taxID.Value == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing required tax ID fields"))
		return
	}

	newInfo, err := p.service.Payments().AddTaxID(ctx, taxID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		status := http.StatusInternalServerError
		if payments.ErrInvalidTaxID.Has(err) {
			status = http.StatusBadRequest
		}
		web.ServeCustomJSONError(ctx, p.log, w, status, err, errs.Unwrap(err).Error())
		return
	}

	if err = json.NewEncoder(w).Encode(newInfo); err != nil {
		p.log.Error("failed encode billing information", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// RemoveTaxID adds a tax ID to a user.
func (p *Payments) RemoveTaxID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var id string
	var ok bool
	if id, ok = mux.Vars(r)["taxID"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("tax ID is required"))
		return

	}

	newInfo, err := p.service.Payments().RemoveTaxID(ctx, id)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		web.ServeCustomJSONError(ctx, p.log, w, http.StatusInternalServerError, err, errs.Unwrap(err).Error())
		return
	}

	if err = json.NewEncoder(w).Encode(newInfo); err != nil {
		p.log.Error("failed encode billing information", zap.Error(ErrPaymentsAPI.Wrap(err)))
	}
}

// serveJSONError writes JSON error to response output stream.
func (p *Payments) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, p.log, w, status, err)
}
