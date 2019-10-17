// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console"
)

var mon = monkit.Package()

// Payments is an api controller that exposes all payment related functionality
type Payments struct {
	log     *zap.Logger
	service *console.Service
}

// NewPayments is a constructor for api payments controller.
func NewPayments(log *zap.Logger, service *console.Service) *Payments {
	return &Payments{
		log:     log,
		service: service,
	}
}

// SetupAccount creates a payment account for the user.
func (p *Payments) SetupAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx = p.authorize(ctx, r)

	err = p.service.Payments().SetupAccount(ctx)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// AccountBalance returns an integer amount in cents that represents the current balance of payment account.
func (p *Payments) AccountBalance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx = p.authorize(ctx, r)

	balance, err := p.service.Payments().AccountBalance(ctx)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}

	var balanceResponse struct {
		Balance int64 `json:"balance"`
	}

	balanceResponse.Balance = balance

	err = json.NewEncoder(w).Encode(balanceResponse)
	if err != nil {
		p.log.Error("failed to write json balance response", zap.Error(err))
	}
}

// AddCreditCard is used to save new credit card and attach it to payment account.
func (p *Payments) AddCreditCard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ctx = p.authorize(ctx, r)

	var requestBody struct {
		Token string `json:"token"`
	}

	decoder := json.NewDecoder(r.Body)

	err = decoder.Decode(&requestBody)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	err = p.service.Payments().AddCreditCard(ctx, requestBody.Token)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (p *Payments) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		p.log.Error("failed to write json error response", zap.Error(err))
	}
}

// authorize checks request for authorization token, validates it and updates context with auth data.
func (p *Payments) authorize(ctx context.Context, r *http.Request) context.Context {
	authHeaderValue := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeaderValue, "Bearer ")

	auth, err := p.service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
	if err != nil {
		return console.WithAuthFailure(ctx, err)
	}

	return console.WithAuth(ctx, auth)
}
