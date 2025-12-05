// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
)

var (
	// ErrAPIKeysAPI - console api keys api error type.
	ErrAPIKeysAPI = errs.Class("console api keys")
)

// APIKeys is an api controller that exposes all api keys related functionality.
type APIKeys struct {
	log     *zap.Logger
	service *console.Service
}

// NewAPIKeys is a constructor for api keys controller.
func NewAPIKeys(log *zap.Logger, service *console.Service) *APIKeys {
	return &APIKeys{
		log:     log,
		service: service,
	}
}

// CreateAPIKey creates new API key for given project.
func (keys *APIKeys) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["projectID"]; !ok {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing projectID route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}
	name := string(bodyBytes)

	err = keys.service.ValidateFreeFormFieldLengths(&name)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	apiKeyVersion := macaroon.APIKeyVersionMin
	if keys.service.GetObjectLockUIEnabled() {
		apiKeyVersion = macaroon.APIKeyVersionObjectLock
	}
	if keys.service.ProjectSupportsAuditableAPIKeys(projectID) {
		apiKeyVersion |= macaroon.APIKeyVersionAuditable
	}

	info, key, err := keys.service.CreateAPIKey(ctx, projectID, name, apiKeyVersion)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	response := console.CreateAPIKeyResponse{
		Key:     key.Serialize(),
		KeyInfo: info,
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		keys.log.Error("failed to write json create api key response", zap.Error(ErrAPIKeysAPI.Wrap(err)))
	}
}

// GetProjectAPIKeys returns paged API keys by project ID.
func (keys *APIKeys) GetProjectAPIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	query := r.URL.Query()

	projectIDParam := query.Get("projectID")
	if projectIDParam == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'projectID' can't be empty"))
		return
	}

	projectID, err := uuid.FromString(projectIDParam)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	limitParam := query.Get("limit")
	if limitParam == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'limit' can't be empty"))
		return
	}

	limit, err := strconv.ParseUint(limitParam, 10, 32)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	pageParam := query.Get("page")
	if pageParam == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'page' can't be empty"))
		return
	}

	page, err := strconv.ParseUint(pageParam, 10, 32)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	orderParam := query.Get("order")
	if orderParam == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'order' can't be empty"))
		return
	}

	order, err := strconv.ParseUint(orderParam, 10, 32)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	orderDirectionParam := query.Get("orderDirection")
	if orderDirectionParam == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'orderDirection' can't be empty"))
		return
	}

	orderDirection, err := strconv.ParseUint(orderDirectionParam, 10, 32)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	searchString := query.Get("search")

	cursor := console.APIKeyCursor{
		Search:         searchString,
		Limit:          uint(limit),
		Page:           uint(page),
		Order:          console.APIKeyOrder(order),
		OrderDirection: console.OrderDirection(orderDirection),
	}

	apiKeys, err := keys.service.GetAPIKeys(ctx, projectID, cursor)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(apiKeys)
	if err != nil {
		keys.log.Error("failed to write json all api keys response", zap.Error(ErrAPIKeysAPI.Wrap(err)))
	}
}

// GetAllAPIKeyNames returns all API key names by project ID.
func (keys *APIKeys) GetAllAPIKeyNames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	projectIDString := r.URL.Query().Get("projectID")
	if projectIDString == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("Project ID was not provided."))
		return
	}

	projectID, err := uuid.FromString(projectIDString)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	apiKeyNames, err := keys.service.GetAllAPIKeyNamesByProjectID(ctx, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(apiKeyNames)
	if err != nil {
		keys.log.Error("failed to write json all api key names response", zap.Error(ErrAPIKeysAPI.Wrap(err)))
	}
}

// DeleteByIDs deletes API keys by given IDs.
func (keys *APIKeys) DeleteByIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data struct {
		IDs []string `json:"ids"`
	}

	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var keyIDs []uuid.UUID
	for _, id := range data.IDs {
		keyID, err := uuid.FromString(id)
		if err != nil {
			keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}

		keyIDs = append(keyIDs, keyID)
	}

	err = keys.service.DeleteAPIKeys(ctx, keyIDs)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrForbidden.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// DeleteByNameAndProjectID deletes specific API key by it's name and project ID.
// ID here may be project.publicID or project.ID.
func (keys *APIKeys) DeleteByNameAndProjectID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	name := r.URL.Query().Get("name")
	projectIDString := r.URL.Query().Get("projectID")
	publicIDString := r.URL.Query().Get("publicID")

	if name == "" {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var projectID uuid.UUID
	if projectIDString != "" {
		projectID, err = uuid.FromString(projectIDString)
		if err != nil {
			keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	} else if publicIDString != "" {
		projectID, err = uuid.FromString(publicIDString)
		if err != nil {
			keys.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	} else {
		keys.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("Project ID was not provided."))
		return
	}

	err = keys.service.DeleteAPIKeyByNameAndProjectID(ctx, name, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrNoAPIKey.Has(err) {
			keys.serveJSONError(ctx, w, http.StatusNoContent, err)
			return
		}

		keys.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (keys *APIKeys) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, keys.log, w, status, err)
}
