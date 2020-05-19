// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
)

func (server *Server) addCoupon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusInternalServerError)
		return
	}

	var input struct {
		UserID      uuid.UUID `json:"userId"`
		Duration    int       `json:"duration"`
		Amount      int64     `json:"amount"`
		Description string    `json:"description"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal request: %v", err), http.StatusBadRequest)
		return
	}
	switch {
	case input.Duration == 0:
		http.Error(w, "Duration is not set", http.StatusBadRequest)
		return
	case input.Amount == 0:
		http.Error(w, "Amount is not set", http.StatusBadRequest)
		return
	case input.Description == "":
		http.Error(w, "Description is not set", http.StatusBadRequest)
		return
	case input.UserID.IsZero():
		http.Error(w, "UserID is not set", http.StatusBadRequest)
		return
	}

	coupon, err := server.db.StripeCoinPayments().Coupons().Insert(ctx, payments.Coupon{
		UserID:      input.UserID,
		Amount:      input.Amount,
		Duration:    input.Duration,
		Description: input.Description,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to insert coupon: %v", err), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(coupon.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) couponInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	id, ok := vars["couponid"]
	if !ok {
		http.Error(w, "couponId missing", http.StatusBadRequest)
		return
	}

	couponID, err := uuid.FromString(id)
	if err != nil {
		http.Error(w, "invalid couponId", http.StatusBadRequest)
	}

	coupon, err := server.db.StripeCoinPayments().Coupons().Get(ctx, couponID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, fmt.Sprintf("coupon with id %q not found", couponID), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get coupon %q: %v", couponID, err), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(coupon)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}
