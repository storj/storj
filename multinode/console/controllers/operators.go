// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package controllers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/multinode/operators"
)

var (
	// ErrOperators is an internal error type for operators web api controller.
	ErrOperators = errs.Class("nodes web api controller")
)

const (
	defaultLimit = 5
)

// Operators is a web api controller.
type Operators struct {
	log     *zap.Logger
	service *operators.Service
}

// NewOperators is a constructor for Operators.
func NewOperators(log *zap.Logger, service *operators.Service) *Operators {
	return &Operators{
		log:     log,
		service: service,
	}
}

// ListPaginated handles retrieval of operators.
func (controller *Operators) ListPaginated(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Add("Content-Type", "application/json")

	limit := int64(defaultLimit)
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		limit, err = strconv.ParseInt(limitParam, 10, 64)
		if err != nil {
			controller.serveError(w, http.StatusBadRequest, ErrOperators.Wrap(err))
		}
	}

	pageParam := r.URL.Query().Get("page")
	if pageParam == "" {
		controller.serveError(w, http.StatusBadRequest, ErrOperators.Wrap(errs.New("page is missing")))
		return
	}
	pageNumber, err := strconv.ParseInt(pageParam, 10, 64)
	if err != nil {
		controller.serveError(w, http.StatusBadRequest, ErrOperators.Wrap(err))
		return
	}

	cursor := operators.Cursor{
		Limit: limit,
		Page:  pageNumber,
	}
	page, err := controller.service.ListPaginated(ctx, cursor)
	if err != nil {
		controller.log.Error("could not get operators page", zap.Error(ErrOperators.Wrap(err)))
		controller.serveError(w, http.StatusInternalServerError, ErrOperators.Wrap(err))
		return
	}

	if err = json.NewEncoder(w).Encode(page); err != nil {
		controller.log.Error("failed to write json response", zap.Error(ErrOperators.Wrap(err)))
		return
	}
}

// serveError set http statuses and send json error.
func (controller *Operators) serveError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	var response struct {
		Error string `json:"error"`
	}
	response.Error = err.Error()
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		controller.log.Error("failed to write json error response", zap.Error(ErrOperators.Wrap(err)))
	}
}
