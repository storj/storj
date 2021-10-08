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
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
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
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}
	switch {
	case input.Duration == 0:
		sendJSONError(w, "Duration is not set",
			"", http.StatusBadRequest)
		return
	case input.Amount == 0:
		sendJSONError(w, "Amount is not set",
			"", http.StatusBadRequest)
		return
	case input.Description == "":
		sendJSONError(w, "Description is not set",
			"", http.StatusBadRequest)
		return
	case input.UserID.IsZero():
		sendJSONError(w, "UserID is not set",
			"", http.StatusBadRequest)
		return
	}

	coupon, err := server.db.StripeCoinPayments().Coupons().Insert(ctx, payments.CouponOld{
		UserID:      input.UserID,
		Amount:      input.Amount,
		Duration:    &input.Duration,
		Description: input.Description,
	})
	if err != nil {
		sendJSONError(w, "failed to insert coupon",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(coupon.ID)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) couponInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	id, ok := vars["couponid"]
	if !ok {
		sendJSONError(w, "couponId missing",
			"", http.StatusBadRequest)
		return
	}

	couponID, err := uuid.FromString(id)
	if err != nil {
		sendJSONError(w, "invalid couponId",
			"", http.StatusBadRequest)
	}

	coupon, err := server.db.StripeCoinPayments().Coupons().Get(ctx, couponID)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("coupon with id %q not found", couponID),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get coupon",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(coupon)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) deleteCoupon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	UUIDString, ok := vars["couponid"]
	if !ok {
		sendJSONError(w, "couponid missing",
			"", http.StatusBadRequest)
		return
	}

	couponID, err := uuid.FromString(UUIDString)
	if err != nil {
		sendJSONError(w, "invalid couponid",
			err.Error(), http.StatusBadRequest)
		return
	}

	err = server.db.StripeCoinPayments().Coupons().Delete(ctx, couponID)
	if err != nil {
		sendJSONError(w, "unable to delete coupon",
			err.Error(), http.StatusInternalServerError)
		return
	}
}
