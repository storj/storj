// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

func (server *Server) createOAuthClient(w http.ResponseWriter, r *http.Request) {
	oauthClient := console.OAuthClient{}
	err := json.NewDecoder(r.Body).Decode(&oauthClient)
	if err != nil {
		sendJSONError(w, "invalid json", err.Error(), http.StatusBadRequest)
		return
	}

	validID := oauthClient.ID.String() != ""
	validSecret := len(oauthClient.Secret) > 0
	validRedirectURL := oauthClient.RedirectURL != ""
	validUserID := oauthClient.UserID.String() != ""

	if !validID || !validSecret || !validRedirectURL || !validUserID {
		sendJSONError(w, "", "missing required value", http.StatusBadRequest)
		return
	}

	err = server.db.Console().OAuthClients().Create(r.Context(), oauthClient)
	if err != nil {
		sendJSONError(w, "failed to create client", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (server *Server) updateOAuthClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.FromString(mux.Vars(r)["id"])
	if err != nil {
		sendJSONError(w, "missing required client id", err.Error(), http.StatusBadRequest)
		return
	}

	oauthClient := console.OAuthClient{}
	err = json.NewDecoder(r.Body).Decode(&oauthClient)
	if err != nil {
		sendJSONError(w, "invalid json", err.Error(), http.StatusBadRequest)
		return
	}

	oauthClient.ID = id

	err = server.db.Console().OAuthClients().Update(r.Context(), oauthClient)
	if err != nil {
		sendJSONError(w, "failed to update client", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (server *Server) deleteOAuthClient(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.FromString(mux.Vars(r)["id"])
	if err != nil {
		sendJSONError(w, "missing required client id", err.Error(), http.StatusBadRequest)
		return
	}

	err = server.db.Console().OAuthClients().Delete(r.Context(), id)
	if err != nil {
		sendJSONError(w, "failed to delete client", err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
