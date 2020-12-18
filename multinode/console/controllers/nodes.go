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

// Add handles node addition.
func (controller *Nodes) Add(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	var payload struct {
		ID            string `json:"id"`
		APISecret     string `json:"apiSecret"`
		PublicAddress string `json:"publicAddress"`
	}

	if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	id, err := storj.NodeIDFromString(payload.ID)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	apiSecret, err := nodes.APISecretFromBase64(payload.APISecret)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	if err = controller.service.Add(ctx, id, apiSecret, payload.PublicAddress); err != nil {
		// TODO: add more error checks in future, like bad payload if address is invalid or unauthorized if secret invalid.
		controller.log.Error("add node internal error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}
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

	var payload struct {
		Name string `json:"name"`
	}

	if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	err = controller.service.UpdateName(ctx, id, payload.Name)
	if err != nil {
		// TODO: add more error checks in future, like not found if node is missing.
		controller.log.Error("update node name internal error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}
}

// Get handles retrieving node by id.
func (controller *Nodes) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	vars := mux.Vars(r)

	nodeID, err := storj.NodeIDFromString(vars["id"])
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrNodes.Wrap(err))
		return
	}

	node, err := controller.service.Get(ctx, nodeID)
	if err != nil {
		controller.log.Error("get node not found error", zap.Error(err))
		controller.serveError(w, http.StatusNotFound, ErrNodes.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(node); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
		return
	}
}

// List handles retrieving list of nodes.
func (controller *Nodes) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	nodes, err := controller.service.List(ctx)
	if err != nil {
		controller.log.Error("list nodes internal error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(nodes); err != nil {
		controller.log.Error("failed to write json response", zap.Error(err))
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
		// TODO: add more error checks in future, like not found if node is missing.
		controller.log.Error("delete node internal error", zap.Error(err))
		controller.serveError(w, http.StatusInternalServerError, ErrNodes.Wrap(err))
		return
	}
}

// serveError set http statuses and send json error.
func (controller *Nodes) serveError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		controller.log.Error("failed to write json error response", zap.Error(err))
	}
}
