// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/metabase"
)

// BucketInfo contains information about a bucket.
type BucketInfo struct {
	Name         string    `json:"name"`
	UserAgent    string    `json:"userAgent"`
	Placement    string    `json:"placement"`
	Storage      float64   `json:"storage"`
	Egress       float64   `json:"egress"`
	SegmentCount int64     `json:"segmentCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

// BucketInfoPage contains a paginated list of buckets.
type BucketInfoPage struct {
	Items []BucketInfo `json:"items"`

	Limit  uint   `json:"limit"`
	Offset uint64 `json:"offset"`

	PageCount   uint   `json:"pageCount"`
	CurrentPage uint   `json:"currentPage"`
	TotalCount  uint64 `json:"totalCount"`
}

// BucketState contains the state of a bucket.
type BucketState struct {
	Empty bool `json:"empty"`
}

// UpdateBucketRequest contains the fields that can be updated in a project.
type UpdateBucketRequest struct {
	UserAgent *string                    `json:"userAgent"`
	Placement *storj.PlacementConstraint `json:"placement"`

	Reason string `json:"reason"` // Reason for the change, for audit logging
}

// GetProjectBuckets retrieves all buckets for a given project public ID.
func (s *Service) GetProjectBuckets(ctx context.Context, publicID uuid.UUID, search, pageStr, limitStr string, since, before time.Time) (*BucketInfoPage, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	project, err := s.consoleDB.Projects().GetByPublicID(ctx, publicID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}
	// convert page and limit to uint
	limit, err := strconv.ParseUint(limitStr, 10, 32)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("invalid limit"),
		}
	}
	if limit == 0 || limit > 100 {
		limit = 100
	}

	page, err := strconv.ParseUint(pageStr, 10, 32)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("invalid page"),
		}
	}
	if page == 0 {
		page = 1
	}

	if search == "-" {
		// to avoid the gen API requiring that
		// a parameter be non-empty.
		search = ""
	}
	cursor := accounting.BucketUsageCursor{
		Search: search,
		Limit:  uint(limit),
		Page:   uint(page),
	}
	bucketPage, err := s.accountingDB.GetBucketTotals(ctx, project.ID, cursor, since, before)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	infoPage := &BucketInfoPage{
		Items:       make([]BucketInfo, len(bucketPage.BucketUsages)),
		Limit:       uint(limit),
		Offset:      bucketPage.Offset,
		PageCount:   bucketPage.PageCount,
		CurrentPage: bucketPage.CurrentPage,
		TotalCount:  bucketPage.TotalCount,
	}

	if len(bucketPage.BucketUsages) == 0 {
		return infoPage, api.HTTPError{}
	}

	getPlacementName := func(pc storj.PlacementConstraint) string {
		for id, p := range s.placement {
			if id == pc {
				return p.Name
			}
		}
		return "unknown placement"
	}

	for i, bucket := range bucketPage.BucketUsages {
		infoPage.Items[i] = BucketInfo{
			Name:         bucket.BucketName,
			UserAgent:    string(bucket.UserAgent),
			Placement:    getPlacementName(bucket.DefaultPlacement),
			Storage:      bucket.Storage,
			Egress:       bucket.Egress,
			SegmentCount: bucket.SegmentCount,
			CreatedAt:    bucket.CreatedAt,
		}
	}

	return infoPage, api.HTTPError{}
}

// UpdateBucket updates a bucket's user agent, and placement if the bucket is empty.
func (s *Service) UpdateBucket(ctx context.Context, authInfo *AuthInfo, projectPublicID uuid.UUID, bucketName string, req UpdateBucketRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiErr := s.validateUpdateBucketRequest(authInfo, req)
	if apiErr.Err != nil {
		return apiErr
	}

	project, err := s.consoleDB.Projects().GetByPublicID(ctx, projectPublicID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	bucket, err := s.buckets.GetBucket(ctx, []byte(bucketName), project.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if buckets.ErrBucketNotFound.Has(err) {
			status = http.StatusNotFound
			err = errs.New("bucket not found")
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}
	before := bucket

	if req.UserAgent != nil {
		bucket.UserAgent = []byte(*req.UserAgent)
	}
	if req.Placement != nil {
		bucket.Placement = *req.Placement
	}

	updated, err := s.buckets.UpdateBucket(ctx, bucket)
	if err != nil {
		status := http.StatusInternalServerError
		if buckets.ErrBucketNotEmpty.Has(err) {
			status = http.StatusConflict
			err = errs.New("cannot change placement of non-empty bucket")
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     project.OwnerID,
		ProjectID:  &project.PublicID,
		BucketName: &bucket.Name,
		Action:     "update_bucket",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeBucket,
		Reason:     req.Reason,
		Before:     before,
		After:      updated,
		Timestamp:  s.nowFn(),
	})

	return api.HTTPError{}
}

func (s *Service) validateUpdateBucketRequest(authInfo *AuthInfo, req UpdateBucketRequest) api.HTTPError {
	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if authInfo == nil || len(authInfo.Groups) == 0 {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	groups := authInfo.Groups
	hasPerm := func(perm Permission) bool {
		for _, g := range groups {
			if s.authorizer.HasPermissions(g, perm) {
				return true
			}
		}
		return false
	}

	if req.Placement != nil {
		if !hasPerm(PermBucketSetDataPlacement) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change bucket placement"))
		}
		if _, ok := s.placement[*req.Placement]; !ok {
			return apiError(http.StatusBadRequest, errs.New("invalid placement"))
		}
	}

	if req.UserAgent != nil && !hasPerm(PermBucketSetUserAgent) {
		return apiError(http.StatusForbidden, errs.New("not authorized to change bucket user agent"))
	}

	if req.Reason == "" {
		return api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("reason is required"),
		}
	}

	return api.HTTPError{}
}

// GetBucketState retrieves the state of a bucket. The state here includes states that are not
// in the buckets table and requires additional checks, such as whether the bucket is empty.
func (s *Service) GetBucketState(ctx context.Context, projectPublicID uuid.UUID, bucketName string) (*BucketState, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	project, err := s.consoleDB.Projects().GetByPublicID(ctx, projectPublicID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	bucket, err := s.buckets.GetBucket(ctx, []byte(bucketName), project.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if buckets.ErrBucketNotFound.Has(err) {
			status = http.StatusNotFound
			err = errs.New("bucket not found")
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	isEmpty, err := s.metabase.BucketEmpty(ctx, metabase.BucketEmpty{
		ProjectID:  project.ID,
		BucketName: metabase.BucketName(bucket.Name),
	})
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	return &BucketState{
		Empty: isEmpty,
	}, api.HTTPError{}
}
