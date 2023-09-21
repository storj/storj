// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

const (
	missingParamErrMsg = "missing '%s' query parameter"
	invalidParamErrMsg = "invalid value '%s' for query parameter '%s': %w"
)

var (
	// ErrBucketsAPI - console buckets api error type.
	ErrBucketsAPI = errs.Class("console api buckets")
)

// Buckets is an api controller that exposes all buckets related functionality.
type Buckets struct {
	log     *zap.Logger
	service *console.Service
}

// NewBuckets is a constructor for api buckets controller.
func NewBuckets(log *zap.Logger, service *console.Service) *Buckets {
	return &Buckets{
		log:     log,
		service: service,
	}
}

// AllBucketNames returns all bucket names for a specific project.
func (b *Buckets) AllBucketNames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	projectIDString := r.URL.Query().Get("projectID")
	publicIDString := r.URL.Query().Get("publicID")

	var projectID uuid.UUID
	if projectIDString != "" {
		projectID, err = uuid.FromString(projectIDString)
		if err != nil {
			b.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	} else if publicIDString != "" {
		projectID, err = uuid.FromString(publicIDString)
		if err != nil {
			b.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}
	} else {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("Project ID was not provided."))
		return
	}

	bucketNames, err := b.service.GetAllBucketNames(ctx, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			b.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		b.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(bucketNames)
	if err != nil {
		b.log.Error("failed to write json all bucket names response", zap.Error(ErrBucketsAPI.Wrap(err)))
	}
}

// GetBucketTotals returns a page of bucket usage totals since project creation.
func (b *Buckets) GetBucketTotals(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	projectIDString := r.URL.Query().Get("projectID")
	if projectIDString == "" {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(missingParamErrMsg, "projectID"))
		return
	}
	projectID, err := uuid.FromString(projectIDString)
	if err != nil {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(invalidParamErrMsg, projectIDString, "projectID", err))
		return
	}

	beforeString := r.URL.Query().Get("before")
	if beforeString == "" {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(missingParamErrMsg, "before"))
		return
	}
	before, err := time.Parse(dateLayout, beforeString)
	if err != nil {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(invalidParamErrMsg, beforeString, "before", err))
		return
	}

	limitString := r.URL.Query().Get("limit")
	if limitString == "" {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(missingParamErrMsg, "limit"))
		return
	}
	limitU64, err := strconv.ParseUint(limitString, 10, 32)
	if err != nil {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(invalidParamErrMsg, limitString, "limit", err))
		return
	}
	limit := uint(limitU64)

	pageString := r.URL.Query().Get("page")
	if pageString == "" {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(missingParamErrMsg, "page"))
		return
	}
	pageU64, err := strconv.ParseUint(pageString, 10, 32)
	if err != nil {
		b.serveJSONError(ctx, w, http.StatusBadRequest, errs.New(invalidParamErrMsg, pageString, "page", err))
		return
	}
	page := uint(pageU64)

	totals, err := b.service.GetBucketTotals(ctx, projectID, accounting.BucketUsageCursor{
		Limit:  limit,
		Search: r.URL.Query().Get("search"),
		Page:   page,
	}, before)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			b.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		b.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(totals)
	if err != nil {
		b.log.Error("failed to write json bucket totals response", zap.Error(ErrBucketsAPI.Wrap(err)))
	}
}

// serveJSONError writes JSON error to response output stream.
func (b *Buckets) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, b.log, w, status, err)
}
