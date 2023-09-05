// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// Error is default error class for admin package.
var Error = errs.Class("admin")

func sendJSONError(w http.ResponseWriter, errMsg, detail string, statusCode int) {
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

// uuidFromString converts a hex string into a UUID type. It works regardless of whether the string version contains `-` characters.
func uuidFromString(uuidString string) (id uuid.UUID, err error) {
	if len(uuidString) == len(uuid.UUID{}.String()) {
		id, err = uuid.FromString(uuidString)
		if err != nil {
			return id, Error.Wrap(err)
		}
	} else {
		// this case means that dashes may not have been included in the ID passed in
		// to parse, decode from hex, and create UUID from bytes
		b, err := hex.DecodeString(uuidString)
		if err != nil {
			return id, Error.Wrap(err)
		}
		id, err = uuid.FromBytes(b)
		if err != nil {
			return id, Error.Wrap(err)
		}
	}
	return id, nil
}
