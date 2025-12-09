// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/payments"
)

// NullableLimitValue is the value used to indicate that a limit should be set to null.
const NullableLimitValue = -1

// Project contains the information and configurations of a project.
type Project struct {
	ID               uuid.UUID                 `json:"-"`
	PublicID         uuid.UUID                 `json:"id"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	UserAgent        string                    `json:"userAgent"`
	Owner            User                      `json:"owner"`
	CreatedAt        time.Time                 `json:"createdAt"`
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`

	// RateLimit is `nil` when satellite applies the configured default rate limit.
	RateLimit *int `json:"rateLimit"`
	// BurstLimit is `nil` when satellite applies the configured default burst limit.
	BurstLimit       *int `json:"burstLimit"`
	RateLimitHead    *int `json:"rateLimitHead"`
	BurstLimitHead   *int `json:"burstLimitHead"`
	RateLimitGet     *int `json:"rateLimitGet"`
	BurstLimitGet    *int `json:"burstLimitGet"`
	RateLimitPut     *int `json:"rateLimitPut"`
	BurstLimitPut    *int `json:"burstLimitPut"`
	RateLimitDelete  *int `json:"rateLimitDelete"`
	BurstLimitDelete *int `json:"burstLimitDelete"`
	RateLimitList    *int `json:"rateLimitList"`
	BurstLimitList   *int `json:"burstLimitList"`
	// Maxbuckets is `nil` when satellite applies the configured default max buckets.
	MaxBuckets *int `json:"maxBuckets"`
	ProjectUsageLimits[*int64]
	Status       *ProjectStatusInfo   `json:"status"`
	Entitlements *ProjectEntitlements `json:"entitlements"`
}

// ProjectUsageLimits holds project usage limits and current usage. It uses generics for allowing
// to report the limits fields with nil values when they are read from the DB projects table.
//
// StorageUsed and SegmentUsed are nil if there was an error connecting to the Redis
// live accounting cache.
type ProjectUsageLimits[T ~int64 | *int64] struct {
	BandwidthLimit        T      `json:"bandwidthLimit"`
	UserSetBandwidthLimit *int64 `json:"userSetBandwidthLimit"`
	BandwidthUsed         int64  `json:"bandwidthUsed"`
	StorageLimit          T      `json:"storageLimit"`
	UserSetStorageLimit   *int64 `json:"userSetStorageLimit"`
	StorageUsed           *int64 `json:"storageUsed"`
	SegmentLimit          T      `json:"segmentLimit"`
	SegmentUsed           *int64 `json:"segmentUsed"`
}

// ProjectLimitsUpdateRequest contains all limit values to be updated.
type ProjectLimitsUpdateRequest struct {
	MaxBuckets     *int   `json:"maxBuckets"`
	StorageLimit   *int64 `json:"storageLimit"`
	BandwidthLimit *int64 `json:"bandwidthLimit"`
	SegmentLimit   *int64 `json:"segmentLimit"`
	RateLimit      *int   `json:"rateLimit"`
	BurstLimit     *int   `json:"burstLimit"`
	// the following limits are nullable; setting them to 0
	// sets them to null in the DB
	UserSetStorageLimit   *int64 `json:"userSetStorageLimit"`
	UserSetBandwidthLimit *int64 `json:"userSetBandwidthLimit"`
	RateLimitHead         *int   `json:"rateLimitHead"`
	BurstLimitHead        *int   `json:"burstLimitHead"`
	RateLimitGet          *int   `json:"rateLimitGet"`
	BurstLimitGet         *int   `json:"burstLimitGet"`
	RateLimitPut          *int   `json:"rateLimitPut"`
	BurstLimitPut         *int   `json:"burstLimitPut"`
	RateLimitDelete       *int   `json:"rateLimitDelete"`
	BurstLimitDelete      *int   `json:"burstLimitDelete"`
	RateLimitList         *int   `json:"rateLimitList"`
	BurstLimitList        *int   `json:"burstLimitList"`

	Reason string `json:"reason"` // reason for audit log
}

// ProjectStatusInfo is used to list the possible project statuses in the UI.
type ProjectStatusInfo struct {
	Name  string                `json:"name"`
	Value console.ProjectStatus `json:"value"`
}

// UpdateProjectRequest contains the fields that can be updated in a project.
type UpdateProjectRequest struct {
	Name             *string                    `json:"name"`
	Description      *string                    `json:"description"`
	UserAgent        *string                    `json:"userAgent"`
	Status           *console.ProjectStatus     `json:"status"`
	DefaultPlacement *storj.PlacementConstraint `json:"defaultPlacement"`

	Reason string `json:"reason"` // Reason for the change, for audit logging
}

// UpdateProjectEntitlementsRequest contains the fields that can be updated in a project's entitlements.
type UpdateProjectEntitlementsRequest struct {
	NewBucketPlacements      []storj.PlacementConstraint           `json:"newBucketPlacements"`
	ComputeAccessToken       *string                               `json:"computeAccessToken"`
	PlacementProductMappings entitlements.PlacementProductMappings `json:"placementProductMappings"`

	// Reason for the change, for audit logging
	Reason string `json:"reason"`
}

// ProjectEntitlements holds a project's entitlements.
type ProjectEntitlements struct {
	NewBucketPlacements      []string                   `json:"newBucketPlacements"`
	ComputeAccessToken       string                     `json:"computeAccessToken"`
	PlacementProductMappings map[string]MiniProductInfo `json:"placementProductMappings"`
}

// DisableProjectRequest contains the fields required to delete a project.
type DisableProjectRequest struct {
	SetPendingDeletion bool   `json:"setPendingDeletion"`
	Reason             string `json:"reason"` // Reason for the deletion, for audit logging
}

// GetProject gets the project info by either private or public ID.
func (s *Service) GetProject(ctx context.Context, id uuid.UUID) (*Project, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	p, err := s.consoleDB.Projects().GetByPublicOrPrivateID(ctx, id)
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

	u, err := s.consoleDB.Users().Get(ctx, p.OwnerID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			// If user doesn't exist, then the project doesn't exist either.
			status = http.StatusNotFound
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	bandwidthu, storageu, segmentu, apiErr := s.getProjectUsage(ctx, p.ID)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	userAgent := ""
	if p.UserAgent != nil {
		userAgent = string(p.UserAgent)
	}

	var bandwidthl *int64
	if p.BandwidthLimit != nil {
		l := p.BandwidthLimit.Int64()
		bandwidthl = &l
	}

	var storagel *int64
	if p.StorageLimit != nil {
		l := p.StorageLimit.Int64()
		storagel = &l
	}
	var userStoragel *int64
	if p.UserSpecifiedStorageLimit != nil {
		l := p.UserSpecifiedStorageLimit.Int64()
		userStoragel = &l
	}
	var userBandwidthl *int64
	if p.UserSpecifiedBandwidthLimit != nil {
		l := p.UserSpecifiedBandwidthLimit.Int64()
		userBandwidthl = &l
	}

	var status *ProjectStatusInfo
	if p.Status != nil {
		status = &ProjectStatusInfo{Name: p.Status.String(), Value: *p.Status}
	}

	var ents *ProjectEntitlements
	feats, err := s.entitlements.Projects().GetByPublicID(ctx, p.PublicID)
	if err != nil && !entitlements.ErrNotFound.Has(err) {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	} else if err == nil {
		ents, apiErr = s.toProjectEntitlements(feats)
		if apiErr.Err != nil {
			return nil, apiErr
		}
	}

	return &Project{
		ID:          p.ID,
		PublicID:    p.PublicID,
		Name:        p.Name,
		Description: p.Description,
		UserAgent:   userAgent,
		Owner: User{
			ID:       p.OwnerID,
			FullName: u.FullName,
			Email:    u.Email,
		},
		CreatedAt:        p.CreatedAt,
		DefaultPlacement: p.DefaultPlacement,
		RateLimit:        p.RateLimit,
		BurstLimit:       p.BurstLimit,
		RateLimitList:    p.RateLimitList,
		BurstLimitList:   p.BurstLimitList,
		RateLimitHead:    p.RateLimitHead,
		BurstLimitHead:   p.BurstLimitHead,
		RateLimitGet:     p.RateLimitGet,
		BurstLimitGet:    p.BurstLimitGet,
		RateLimitPut:     p.RateLimitPut,
		BurstLimitPut:    p.BurstLimitPut,
		RateLimitDelete:  p.RateLimitDelete,
		BurstLimitDelete: p.BurstLimitDelete,
		MaxBuckets:       p.MaxBuckets,
		ProjectUsageLimits: ProjectUsageLimits[*int64]{
			BandwidthLimit:        bandwidthl,
			UserSetBandwidthLimit: userBandwidthl,
			BandwidthUsed:         bandwidthu,
			StorageLimit:          storagel,
			UserSetStorageLimit:   userStoragel,
			StorageUsed:           storageu,
			SegmentLimit:          p.SegmentLimit,
			SegmentUsed:           segmentu,
		},
		Status:       status,
		Entitlements: ents,
	}, api.HTTPError{}
}

// getProjectLimits returns the project's limits returning the default limits when the limits are
// `nil` in the DB projects table.
//
// id is the ID of a project (NOT the public ID).
func (s *Service) getProjectLimits(ctx context.Context, id uuid.UUID) (bandwidth, storage, segment int64, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	// We return status 409 in the rare case that a project is deleted
	// before its limits can be obtained.
	makeDBErr := func(err error) api.HTTPError {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	bandwidthl, err := s.accounting.GetProjectBandwidthLimit(ctx, id)
	if err != nil {
		return 0, 0, 0, makeDBErr(err)
	}

	storagel, err := s.accounting.GetProjectStorageLimit(ctx, id)
	if err != nil {
		return 0, 0, 0, makeDBErr(err)
	}

	segmentl, err := s.accounting.GetProjectSegmentLimit(ctx, id)
	if err != nil {
		return 0, 0, 0, makeDBErr(err)
	}

	return bandwidthl.Int64(), storagel.Int64(), segmentl.Int64(), api.HTTPError{}
}

// getProjectUsage returns the project's usage and limits. If there is an error connecting to Redis
// live accounting cache, some of the usage fields are nil and it will be reported as a warning log
// message.
//
// id is the ID of a project (NOT the public ID).
func (s *Service) getProjectUsage(
	ctx context.Context,
	id uuid.UUID,
) (bandwidth int64, storage, segment *int64, _ api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	bandwidt, err := s.accounting.GetProjectBandwidthTotals(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return 0, nil, nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	var cacheErrs []error
	handleLiveAccountingErr := func(err error) *api.HTTPError {
		if accounting.ErrSystemOrNetError.Has(err) {
			cacheErrs = append(cacheErrs, err)
			return nil
		}

		return &api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	// GetProjectStorageAndSegmentUsage uses the live accounting, so we need to check if
	// there is an error connecting to it.
	storageUsage, segmentUsage, err := s.accounting.GetProjectStorageAndSegmentUsage(ctx, id)
	if err != nil {
		if aerr := handleLiveAccountingErr(err); aerr != nil {
			return 0, nil, nil, *aerr
		}
	} else {
		storage = &storageUsage
		segment = &segmentUsage
	}

	if len(cacheErrs) != 0 {
		s.log.Warn(
			"Error getting project usage data from live accounting cache",
			zap.Errors("errors", cacheErrs),
		)
	}

	return bandwidt, storage, segment, api.HTTPError{}
}

// UpdateProjectLimits updates the project's max buckets, storage, bandwidth, segment, rate, and burst limits.
func (s *Service) UpdateProjectLimits(ctx context.Context, authInfo *AuthInfo, id uuid.UUID, req ProjectLimitsUpdateRequest) (*Project, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if authInfo == nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.New("not authorized"),
		}
	}

	if req.Reason == "" {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("reason is required"),
		}
	}

	p, err := s.consoleDB.Projects().GetByPublicID(ctx, id)
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

	toUpdate, err := s.validateProjectLimitRequest(p, req)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.Wrap(err),
		}
	}

	err = s.consoleDB.Projects().UpdateLimitsGeneric(ctx, p.ID, toUpdate)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	afterState, err := s.consoleDB.Projects().GetByPublicID(ctx, id)
	if err != nil {
		s.log.Error("Failed to fetch project after updating limits", zap.Error(err))
	} else {
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     p.OwnerID,
			ProjectID:  &p.PublicID,
			Action:     "update_project_limits",
			AdminEmail: authInfo.Email,
			ItemType:   changehistory.ItemTypeProject,
			Reason:     req.Reason,
			Before:     p,
			After:      afterState,
			Timestamp:  s.nowFn(),
		})
	}

	return s.GetProject(ctx, id)
}

func (s *Service) validateProjectLimitRequest(p *console.Project, req ProjectLimitsUpdateRequest) (toUpdate []console.Limit, err error) {
	var errGroup errs.Group
	intTo64 := func(i *int) *int64 {
		if i == nil {
			return nil
		}
		v := int64(*i)
		return &v
	}
	sizeTo64 := func(i *memory.Size) *int64 {
		if i == nil {
			return nil
		}
		i64 := i.Int64()
		return &i64
	}

	limits := []struct {
		requestValue *int64
		currentValue *int64
		limitKind    console.LimitKind
		disallowNull bool
	}{
		{
			requestValue: req.StorageLimit,
			currentValue: sizeTo64(p.StorageLimit),
			limitKind:    console.StorageLimit,
			disallowNull: true,
		}, {
			requestValue: req.BandwidthLimit,
			currentValue: sizeTo64(p.BandwidthLimit),
			limitKind:    console.BandwidthLimit,
			disallowNull: true,
		}, {
			requestValue: req.SegmentLimit,
			currentValue: p.SegmentLimit,
			limitKind:    console.SegmentLimit,
			disallowNull: true,
		}, {
			requestValue: intTo64(req.MaxBuckets),
			currentValue: intTo64(p.MaxBuckets),
			limitKind:    console.BucketsLimit,
		}, {
			requestValue: intTo64(req.RateLimit),
			currentValue: intTo64(p.RateLimit),
			limitKind:    console.RateLimit,
		}, {
			requestValue: intTo64(req.BurstLimit),
			currentValue: intTo64(p.BurstLimit),
			limitKind:    console.BurstLimit,
		}, {
			requestValue: req.UserSetStorageLimit,
			currentValue: sizeTo64(p.UserSpecifiedStorageLimit),
			limitKind:    console.UserSetStorageLimit,
		}, {
			requestValue: req.UserSetBandwidthLimit,
			currentValue: sizeTo64(p.UserSpecifiedBandwidthLimit),
			limitKind:    console.UserSetBandwidthLimit,
		}, {
			requestValue: intTo64(req.RateLimitHead),
			currentValue: intTo64(p.RateLimitHead),
			limitKind:    console.RateLimitHead,
		}, {
			requestValue: intTo64(req.BurstLimitHead),
			currentValue: intTo64(p.BurstLimitHead),
			limitKind:    console.BurstLimitHead,
		}, {
			requestValue: intTo64(req.RateLimitGet),
			currentValue: intTo64(p.RateLimitGet),
			limitKind:    console.RateLimitGet,
		}, {
			requestValue: intTo64(req.BurstLimitGet),
			currentValue: intTo64(p.BurstLimitGet),
			limitKind:    console.BurstLimitGet,
		}, {
			requestValue: intTo64(req.RateLimitPut),
			currentValue: intTo64(p.RateLimitPut),
			limitKind:    console.RateLimitPut,
		}, {
			requestValue: intTo64(req.BurstLimitPut),
			currentValue: intTo64(p.BurstLimitPut),
			limitKind:    console.BurstLimitPut,
		}, {
			requestValue: intTo64(req.RateLimitDelete),
			currentValue: intTo64(p.RateLimitDelete),
			limitKind:    console.RateLimitDelete,
		}, {
			requestValue: intTo64(req.BurstLimitDelete),
			currentValue: intTo64(p.BurstLimitDelete),
			limitKind:    console.BurstLimitDelete,
		}, {
			requestValue: intTo64(req.RateLimitList),
			currentValue: intTo64(p.RateLimitList),
			limitKind:    console.RateLimitList,
		}, {
			requestValue: intTo64(req.BurstLimitList),
			currentValue: intTo64(p.BurstLimitList),
			limitKind:    console.BurstLimitList,
		},
	}
	for _, limit := range limits {
		if limit.requestValue == nil {
			continue
		}

		allowNull := !limit.disallowNull
		if allowNull && *limit.requestValue == NullableLimitValue && limit.currentValue != nil {
			toUpdate = append(toUpdate, console.Limit{Kind: limit.limitKind, Value: nil})
			continue
		}
		if *limit.requestValue < 0 {
			errGroup = append(errGroup, errs.New("%s cannot be negative", limit.limitKind))
			continue
		}
		if limit.currentValue == nil || *limit.requestValue != *limit.currentValue {
			toUpdate = append(toUpdate, console.Limit{Kind: limit.limitKind, Value: limit.requestValue})
		}
	}

	if err = errGroup.Err(); err != nil {
		return nil, err
	}

	return toUpdate, nil
}

// GetProjectStatuses returns the possible project statuses.
func (s *Service) GetProjectStatuses(ctx context.Context) ([]ProjectStatusInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	statuses := make([]ProjectStatusInfo, len(console.ProjectStatuses))
	for i, k := range console.ProjectStatuses {
		statuses[i] = ProjectStatusInfo{
			Name:  k.String(),
			Value: k,
		}
	}
	return statuses, api.HTTPError{}
}

// UpdateProject updates the project's information by public ID.
func (s *Service) UpdateProject(ctx context.Context, authInfo *AuthInfo, publicID uuid.UUID, req UpdateProjectRequest) (*Project, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiErr := s.validateUpdateProjectRequest(ctx, authInfo, req)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	p, err := s.consoleDB.Projects().GetByPublicID(ctx, publicID)
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

	beforeState := *p

	if req.Name != nil {
		p.Name = *req.Name
	}
	if req.Description != nil {
		p.Description = *req.Description
	}
	if req.UserAgent != nil {
		p.UserAgent = []byte(*req.UserAgent)
	}
	if req.Status != nil {
		p.Status = req.Status
	}
	if req.DefaultPlacement != nil {
		p.DefaultPlacement = *req.DefaultPlacement
	}

	err = s.consoleDB.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		if err = tx.Projects().Update(ctx, p); err != nil {
			return err
		}
		if req.DefaultPlacement != nil {
			if err = tx.Projects().UpdateDefaultPlacement(ctx, p.ID, *req.DefaultPlacement); err != nil {
				return err
			}
		}
		if req.UserAgent != nil {
			if err = tx.Projects().UpdateUserAgent(ctx, p.ID, []byte(*req.UserAgent)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     p.OwnerID,
		ProjectID:  &p.PublicID,
		Action:     "update_project",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeProject,
		Reason:     req.Reason,
		Before:     beforeState,
		After:      *p,
		Timestamp:  s.nowFn(),
	})

	return s.GetProject(ctx, publicID)
}

func (s *Service) validateUpdateProjectRequest(ctx context.Context, authInfo *AuthInfo, request UpdateProjectRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

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

	valid := false
	var errGroup errs.Group
	if request.Reason == "" {
		errGroup = append(errGroup, errs.New("reason is required"))
	}

	if request.Status != nil {
		if !hasPerm(PermProjectUpdate) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change project status"))
		}
		if *request.Status == console.ProjectPendingDeletion {
			// this is because setting to pending deletion may lead to data deletion by a chore
			return apiError(http.StatusForbidden, errs.New("not authorized to set project status to pending deletion"))
		}
		for _, ps := range console.ProjectStatuses {
			if *request.Status == ps {
				valid = true
				break
			}
		}
		if !valid {
			errGroup = append(errGroup, errs.New("invalid project status %d", *request.Status))
		}
	}

	if request.Name != nil {
		if !hasPerm(PermProjectUpdate) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change project name"))
		}
		if *request.Name == "" {
			errGroup = append(errGroup, errs.New("name cannot be empty"))
		}
	}

	if request.Description != nil && !hasPerm(PermProjectUpdate) {
		return apiError(http.StatusForbidden, errs.New("not authorized to change project description"))
	}

	if request.UserAgent != nil && !hasPerm(PermProjectSetUserAgent) {
		return apiError(http.StatusForbidden, errs.New("not authorized to set user agent"))
	}

	if request.DefaultPlacement != nil {
		if !hasPerm(PermProjectSetDataPlacement) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change project default placement"))
		}
		placements, _ := s.GetPlacements(ctx)
		placementValid := false
		for _, p := range placements {
			if p.ID == *request.DefaultPlacement {
				placementValid = true
				break
			}
		}
		if !placementValid {
			errGroup = append(errGroup, errs.New("invalid placement ID %d", *request.DefaultPlacement))
		}
	}

	if errGroup != nil {
		return apiError(http.StatusBadRequest, errGroup.Err())
	}

	if request.DefaultPlacement == nil {
		return api.HTTPError{}
	}

	return api.HTTPError{}
}

// DisableProject deletes a project by ID.
func (s *Service) DisableProject(ctx context.Context, authInfo *AuthInfo, id uuid.UUID, request DisableProjectRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if authInfo == nil {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	hasPerm := func(perm ...Permission) bool {
		for _, g := range authInfo.Groups {
			if s.authorizer.HasPermissions(g, perm...) {
				return true
			}
		}
		return false
	}

	if request.SetPendingDeletion {
		if !hasPerm(PermProjectMarkPendingDeletion) {
			return apiError(http.StatusForbidden, errs.New("not authorized to mark project pending deletion"))
		}
		if !s.adminConfig.PendingDeleteProjectCleanupEnabled {
			return apiError(http.StatusConflict, errs.New("abbreviated project deletion is not enabled"))
		}
	} else {
		if !hasPerm(PermProjectDeleteNoData) {
			return apiError(http.StatusForbidden, errs.New("not authorized to disable project"))
		}
	}

	p, err := s.consoleDB.Projects().GetByPublicOrPrivateID(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return apiError(status, err)
	}

	user, err := s.consoleDB.Users().Get(ctx, p.OwnerID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	afterState := *p
	disabledStatus := console.ProjectDisabled
	if request.SetPendingDeletion {
		// for abbreviated project deletion, status is set to ProjectPendingDeletion
		// so a cleanup chore can later finalize the deletion
		disabledStatus = console.ProjectPendingDeletion
	}
	afterState.Status = &disabledStatus

	auditLog := func(action string) {
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     p.OwnerID,
			ProjectID:  &p.PublicID,
			Action:     action,
			AdminEmail: authInfo.Email,
			ItemType:   changehistory.ItemTypeProject,
			Reason:     request.Reason,
			Before:     *p,
			After:      afterState,
			Timestamp:  s.nowFn(),
		})
	}

	// Check if the project should be force deleted
	if s.consoleConfig.SelfServeAccountDeleteEnabled && user.Status == console.UserRequestedDeletion && (user.IsFree() || user.FinalInvoiceGenerated) {
		err = s.forceDisableProject(ctx, p.ID)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}

		auditLog("force_disable_project")

		return api.HTTPError{}
	}

	apiErr := s.checkProjectUsageForDisabling(ctx, user, p)
	if apiErr.Err != nil {
		return apiErr
	}

	if request.SetPendingDeletion {
		err = s.completeProjectDisabling(ctx, p.ID, false)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}

		auditLog("mark_project_pending_deletion")

		return api.HTTPError{}
	}

	// Check for existing buckets
	options := buckets.ListOptions{Limit: 1, Direction: buckets.DirectionForward}
	bucketsList, err := s.buckets.ListBuckets(ctx, p.ID, options, macaroon.AllowedBuckets{All: true})
	if err != nil {
		return api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}
	if len(bucketsList.Items) > 0 {
		return apiError(http.StatusConflict, errs.New("buckets still exist"))
	}

	err = s.completeProjectDisabling(ctx, p.ID, false)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	auditLog("disable_project")

	return api.HTTPError{}
}

func (s *Service) checkProjectUsageForDisabling(ctx context.Context, u *console.User, p *console.Project) api.HTTPError {
	if u.Kind != console.PaidUser {
		return api.HTTPError{}
	}

	// Check project usage status
	_, invoicingIncomplete, _, err := s.payments.CheckProjectUsageStatus(ctx, p.ID, p.PublicID)
	if err != nil {
		if payments.ErrUnbilledUsage.Has(err) {
			return api.HTTPError{
				Status: http.StatusConflict,
				Err:    Error.New("usage for current month exists"),
			}
		}
		return api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}
	if invoicingIncomplete {
		return api.HTTPError{
			Status: http.StatusConflict,
			Err:    Error.New("usage for last month exists but is not billed yet"),
		}
	}

	// Check for open invoice items
	err = s.payments.CheckProjectInvoicingStatus(ctx, p.ID)
	if err != nil {
		return api.HTTPError{
			Status: http.StatusConflict,
			Err:    Error.Wrap(err),
		}
	}

	return api.HTTPError{}
}

// forceDisableProject deletes all of a project's buckets and data.
func (s *Service) forceDisableProject(ctx context.Context, projectID uuid.UUID) error {
	listOptions := buckets.ListOptions{Direction: buckets.DirectionForward}
	allowedBuckets := macaroon.AllowedBuckets{All: true}

	bucketsList, err := s.buckets.ListBuckets(ctx, projectID, listOptions, allowedBuckets)
	if err != nil {
		return err
	}

	if len(bucketsList.Items) > 0 {
		var errList errs.Group
		for _, bucket := range bucketsList.Items {
			bucketLocation := metabase.BucketLocation{ProjectID: projectID, BucketName: metabase.BucketName(bucket.Name)}
			_, err = s.metabase.DeleteAllBucketObjects(ctx, metabase.DeleteAllBucketObjects{
				Bucket: bucketLocation,
			})
			if err != nil {
				errList.Add(err)
				continue
			}

			empty, err := s.metabase.BucketEmpty(ctx, metabase.BucketEmpty{
				ProjectID:  projectID,
				BucketName: metabase.BucketName(bucket.Name),
			})
			if err != nil {
				errList.Add(err)
				continue
			}
			if !empty {
				errList.Add(errs.New("bucket not empty: %s", bucket.Name))
				continue
			}

			err = s.buckets.DeleteBucket(ctx, []byte(bucket.Name), projectID)
			if err != nil {
				errList.Add(err)
			}
		}
		if errList.Err() != nil {
			return errList.Err()
		}
	}

	return s.completeProjectDisabling(ctx, projectID, true)
}

func (s *Service) completeProjectDisabling(ctx context.Context, projectID uuid.UUID, forced bool) error {
	if !forced && s.adminConfig.PendingDeleteProjectCleanupEnabled {
		return s.consoleDB.Projects().UpdateStatus(ctx, projectID, console.ProjectPendingDeletion)
	}

	return s.consoleDB.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		err := tx.APIKeys().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			return err
		}

		err = tx.Domains().DeleteAllByProjectID(ctx, projectID)
		if err != nil {
			s.log.Error("failed to delete all domains for project",
				zap.String("project_id", projectID.String()),
				zap.Error(err),
			)
		}

		err = tx.Projects().UpdateStatus(ctx, projectID, console.ProjectDisabled)
		if err != nil {
			return err
		}

		return nil
	})
}

// UpdateProjectEntitlements updates the entitlements for a project by its public ID.
// Only one of new bucket placements, placement:product mappings and compute access token
// can be updated at a time.
func (s *Service) UpdateProjectEntitlements(ctx context.Context, authInfo *AuthInfo, publicID uuid.UUID, request UpdateProjectEntitlementsRequest) (*ProjectEntitlements, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) (*ProjectEntitlements, api.HTTPError) {
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	fieldCount := 0
	if request.NewBucketPlacements != nil {
		fieldCount++
	}
	if request.PlacementProductMappings != nil {
		fieldCount++
	}
	if request.ComputeAccessToken != nil {
		fieldCount++
	}
	if fieldCount == 0 {
		return apiError(http.StatusBadRequest, errs.New("no fields to update"))
	}
	if fieldCount > 1 {
		return apiError(http.StatusBadRequest, errs.New("only one field can be updated at a time"))
	}

	var errGroup errs.Group

	if request.NewBucketPlacements != nil {
		if len(request.NewBucketPlacements) == 0 {
			errGroup = append(errGroup, errs.New("new bucket placements cannot be empty"))
		}

		for _, placement := range request.NewBucketPlacements {
			if _, exists := s.placement[placement]; !exists {
				errGroup = append(errGroup, errs.New("invalid placement constraint in new bucket placements: %v", placement))
			}
		}
	}

	if request.PlacementProductMappings != nil {
		if len(request.PlacementProductMappings) == 0 {
			errGroup = append(errGroup, errs.New("placement:product mappings cannot be empty"))
		}
		for placement, productID := range request.PlacementProductMappings {
			if _, exists := s.placement[placement]; !exists {
				errGroup = append(errGroup, errs.New("invalid placement constraint in placement:product mapping: %v", placement))
			}
			if _, exists := s.products[productID]; !exists {
				errGroup = append(errGroup, errs.New("invalid product ID in placement:product mapping: %d", productID))
			}
		}
	}

	if errGroup != nil {
		return apiError(http.StatusBadRequest, errGroup.Err())
	}

	p, err := s.consoleDB.Projects().GetByPublicID(ctx, publicID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("project not found")
		}
		return apiError(status, err)
	}

	feats, err := s.entitlements.Projects().GetByPublicID(ctx, publicID)
	if err != nil {
		if entitlements.ErrNotFound.Has(err) {
			feats = entitlements.ProjectFeatures{}
		} else {
			return apiError(http.StatusInternalServerError, err)
		}
	}

	if request.ComputeAccessToken != nil {
		newAccessToken := []byte(*request.ComputeAccessToken)
		if *request.ComputeAccessToken == "" {
			newAccessToken = nil
		}

		err = s.entitlements.Projects().SetComputeAccessTokenByPublicID(ctx, publicID, newAccessToken)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}
	} else if request.NewBucketPlacements != nil {
		err = s.entitlements.Projects().SetNewBucketPlacementsByPublicID(ctx, publicID, request.NewBucketPlacements)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}
	} else if request.PlacementProductMappings != nil {
		err = s.entitlements.Projects().SetPlacementProductMappingsByPublicID(ctx, publicID, request.PlacementProductMappings)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}
	}

	newEntitlements, err := s.entitlements.Projects().GetByPublicID(ctx, publicID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     p.OwnerID,
		ProjectID:  &p.PublicID,
		Action:     "update_project_entitlements",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeProject,
		Reason:     request.Reason,
		Before:     feats,
		After:      newEntitlements,
		Timestamp:  s.nowFn(),
	})

	return s.toProjectEntitlements(newEntitlements)
}

func (s *Service) toProjectEntitlements(feats entitlements.ProjectFeatures) (*ProjectEntitlements, api.HTTPError) {
	mappedProducts := make(map[string]MiniProductInfo)
	for placementID, productID := range feats.PlacementProductMappings {
		productInfo, err := s.getProductByID(productID)
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
		var placement string
		if pc, ok := s.placement[placementID]; ok {
			placement = fmt.Sprintf("(%d) - %s", pc.ID, pc.Name)
		}
		mappedProducts[placement] = productInfo.MiniInfo()
	}
	var computeAccessToken string
	if len(feats.ComputeAccessToken) > 0 {
		computeAccessToken = string(feats.ComputeAccessToken)
	}

	var newBucketPlacements []string
	for _, placement := range feats.NewBucketPlacements {
		if pc, ok := s.placement[placement]; ok {
			newBucketPlacements = append(newBucketPlacements, fmt.Sprintf("(%d) - %s", pc.ID, pc.Name))
		}
	}

	return &ProjectEntitlements{
		NewBucketPlacements:      newBucketPlacements,
		ComputeAccessToken:       computeAccessToken,
		PlacementProductMappings: mappedProducts,
	}, api.HTTPError{}
}

// TestToggleSelfServeAccountDelete is a test helper to toggle self-serve account deletion.
func (s *Service) TestToggleSelfServeAccountDelete(enabled bool) {
	s.consoleConfig.SelfServeAccountDeleteEnabled = enabled
}

// TestToggleAbbreviatedProjectDelete is a test helper to toggle abbreviated project deletion.
func (s *Service) TestToggleAbbreviatedProjectDelete(enabled bool) {
	s.adminConfig.PendingDeleteProjectCleanupEnabled = enabled
}
