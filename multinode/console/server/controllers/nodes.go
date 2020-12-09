// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/multinode/nodes"
)

var (
	// ErrNodes is an internal error type for nodes web api controller.
	ErrNodes = errs.Class("nodes web api controller error")
)

// Nodes is a web api controller.
type Nodes struct {
	log     *zap.Logger
	service *nodes.Service
}

// NewNodes is a constructor for Nodes.
func NewNodes(log *zap.Logger, service *nodes.Service) *Nodes {
	return &Nodes{
		log:     log,
		service: service,
	}
}

// AddNodeRequest holds all data needed to add node.
type AddNodeRequest struct {
	ID            string `json:"id"`
	APISecret     string `json:"apiSecret"`
	PublicAddress string `json:"publicAddress"`
}

// Add handles node addition.
func (controller *Nodes) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error

	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	var request AddNodeRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	id, err := storj.NodeIDFromString(request.ID)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	apiSecret, err := nodes.APISecretFromBase64(request.APISecret)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	if err = controller.service.Add(ctx, id, apiSecret, request.PublicAddress); err != nil {
		// TODO: add more error checks in future, like bad request if address is invalid or unauthorized if secret invalid.
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}
}

// Delete handles node removal.
func (controller *Nodes) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error

	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	segmentParams := mux.Vars(r)

	idString, ok := segmentParams["id"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.New("id segment parameter is missing"))
		return
	}

	id, err := storj.NodeIDFromString(idString)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	if err = controller.service.Remove(ctx, id); err != nil {
		// TODO: add more error checks in future, like not found if node is missing or unauthorized if secret invalid.
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}
}

// UpdateNodeNameRequest holds all data needed to add node.
type UpdateNodeNameRequest struct {
	Name string `json:"name"`
}

// UpdateName is an endpoint to update node name.
func (controller *Nodes) UpdateName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error

	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	segmentParams := mux.Vars(r)

	idString, ok := segmentParams["id"]
	if !ok {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.New("id segment parameter is missing"))
		return
	}

	id, err := storj.NodeIDFromString(idString)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	var request UpdateNodeNameRequest
	if err = json.NewDecoder(r.Body).Decode(&request); err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	err = controller.service.Update(ctx, id, request.Name)
	if err != nil {
		// TODO: add more error checks in future, like not found if node is missing or unauthorized if secret invalid.
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}
}

// serveError is used to log error, set http statuses and send error with json.
func (controller *Nodes) serveError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	controller.log.Error("", zap.Error(err))

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		controller.log.Error("failed to write json error response", zap.Error(err))
	}
}
