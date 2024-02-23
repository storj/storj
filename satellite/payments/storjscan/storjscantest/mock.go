// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscantest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/payments/storjscan"
)

// CheckAuth checks request auth headers against provided id and secret.
func CheckAuth(r *http.Request, identifier, secret string) error {
	user, pass, ok := r.BasicAuth()
	if !ok {
		return errs.New("missing authorization")
	}
	if user != identifier {
		return errs.New("identifier is invalid")
	}
	if pass != secret {
		return errs.New("secret is invalid")
	}
	return nil
}

// ServePayments serves payments to response writer.
func ServePayments(t *testing.T, w http.ResponseWriter, from map[int64]int64, blocks []storjscan.Header, payments []storjscan.Payment) {
	var response struct {
		LatestBlocks []storjscan.Header
		Payments     []storjscan.Payment
	}
	response.LatestBlocks = blocks

	for chainID, lastSeenBlock := range from {
		for _, payment := range payments {
			if payment.ChainID == chainID && payment.BlockNumber >= lastSeenBlock {
				response.Payments = append(response.Payments, payment)
			}
		}
	}

	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		t.Fatal(err)
	}
}

// ServeJSONError serves JSON error to response writer.
func ServeJSONError(t *testing.T, w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		t.Fatal(err)
	}
}
