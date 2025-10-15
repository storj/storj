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
