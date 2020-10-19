// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

func (server *Server) addAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		PartnerID uuid.UUID `json:"partnerId"`
		Name      string    `json:"name"`
	}

	var output struct {
		APIKey string `json:"apikey"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		httpJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.Name == "" {
		httpJSONError(w, "Name is not set",
			"", http.StatusBadRequest)
		return
	}

	_, err = server.db.Console().APIKeys().GetByNameAndProjectID(ctx, input.Name, projectUUID)
	if err == nil {
		httpJSONError(w, "api-key with given name already exists",
			"", http.StatusConflict)
		return
	}

	secret, err := macaroon.NewSecret()
	if err != nil {
		httpJSONError(w, "could not create macaroon secret",
			err.Error(), http.StatusInternalServerError)
		return
	}

	key, err := macaroon.NewAPIKey(secret)
	if err != nil {
		httpJSONError(w, "could not create api-key",
			err.Error(), http.StatusInternalServerError)
		return
	}

	apikey := console.APIKeyInfo{
		Name:      input.Name,
		ProjectID: projectUUID,
		Secret:    secret,
		PartnerID: input.PartnerID,
	}

	_, err = server.db.Console().APIKeys().Create(ctx, key.Head(), apikey)
	if err != nil {
		httpJSONError(w, "unable to add api-key to database",
			err.Error(), http.StatusInternalServerError)
		return
	}

	output.APIKey = key.Serialize()
	data, err := json.Marshal(output)
	if err != nil {
		httpJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) deleteAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	apikeyString, ok := vars["apikey"]
	if !ok {
		httpJSONError(w, "apikey missing",
			"", http.StatusBadRequest)
		return
	}

	apikey, err := macaroon.ParseAPIKey(apikeyString)
	if err != nil {
		httpJSONError(w, "invalid apikey format",
			err.Error(), http.StatusBadRequest)
		return
	}

	info, err := server.db.Console().APIKeys().GetByHead(ctx, apikey.Head())
	if err != nil {
		httpJSONError(w, "could not get apikey id",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.db.Console().APIKeys().Delete(ctx, info.ID)
	if err != nil {
		httpJSONError(w, "unable to delete apikey",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) deleteAPIKeyByName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	apikeyName, ok := vars["name"]
	if !ok {
		httpJSONError(w, "apikey name missing",
			"", http.StatusBadRequest)
		return
	}

	info, err := server.db.Console().APIKeys().GetByNameAndProjectID(ctx, apikeyName, projectUUID)
	if err != nil {
		httpJSONError(w, "could not get apikey id",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.db.Console().APIKeys().Delete(ctx, info.ID)
	if err != nil {
		httpJSONError(w, "unable to delete apikey",
			err.Error(), http.StatusInternalServerError)
		return
	}
}
