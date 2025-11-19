// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func (server *Server) addRESTKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmailAndTenant(ctx, userEmail, nil)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		Expiration string `json:"expiration"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	var expiration time.Duration
	if input.Expiration != "" {
		expiration, err = time.ParseDuration(input.Expiration)
		if err != nil {
			sendJSONError(
				w,
				"failed to parse expiration. It accepts any non-negative value according the format rules of https://pkg.go.dev/time#ParseDuration",
				err.Error(),
				http.StatusBadRequest,
			)
			return
		}

		if expiration < 0 {
			sendJSONError(
				w,
				"invalid expiration value",
				"value must result in a greater or equal than 0 duration",
				http.StatusBadRequest,
			)
		}
	}

	apiKey, expiresAt, err := server.restKeys.CreateNoAuth(ctx, user.ID, &expiration)
	if err != nil {
		sendJSONError(w, "api key creation failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var output struct {
		APIKey    string    `json:"apikey"`
		ExpiresAt time.Time `json:"expiresAt"`
	}

	output.APIKey = apiKey
	output.ExpiresAt = *expiresAt

	data, err := json.Marshal(output)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) revokeRESTKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	apiKey, ok := vars["apikey"]
	if !ok {
		sendJSONError(w, "api key missing",
			"", http.StatusBadRequest)
		return
	}

	err := server.restKeys.RevokeByKeyNoAuth(ctx, apiKey)
	if err != nil {
		sendJSONError(w, "failed to revoke api key",
			err.Error(), http.StatusNotFound)
		return
	}
}
