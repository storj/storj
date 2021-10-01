// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"
)

// Error is default error class for admin package.
var Error = errs.Class("admin")

func httpJSONError(w http.ResponseWriter, errMsg, detail string, statusCode int) {
	errStr := struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}{
		Error:  errMsg,
		Detail: detail,
	}
	body, err := json.Marshal(errStr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sendJSONData(w, statusCode, body)
}

func sendJSONData(w http.ResponseWriter, statusCode int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data) // any error here entitles a client side disconnect or similar, which we do not care about.
}
