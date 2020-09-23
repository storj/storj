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

func httpJSONError(w http.ResponseWriter, error, detail string, statusCode int) {
	errStr := struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}{
		Error:  error,
		Detail: detail,
	}
	byt, err := json.Marshal(errStr)
	if err != nil {
		return
	}

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(byt) // any error here entitles a client side disconnect or similar, which we do not care about.
}
