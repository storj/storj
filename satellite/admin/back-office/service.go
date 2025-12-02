// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
)

// Defaults contains default values for limits which are not stored in the DB.
type Defaults struct {
	MaxBuckets int
	RateLimit  int
}

// Service provides functionality for administrating satellites.
type Service struct {
	log *zap.Logger

	authorizer  *Authorizer
	auditLogger *auditlogger.Logger

	accountingDB accounting.ProjectAccounting
	consoleDB    console.DB
	history      changehistory.DB
	metabase     *metabase.DB

	accountFreeze *console.AccountFreezeService
	accounting    *accounting.Service
	buckets       *buckets.Service
	analytics     *analytics.Service
	entitlements  *entitlements.Service
	restKeys      restapikeys.Service
	payments      payments.Accounts

	placement nodeselection.PlacementDefinitions
	products  map[int32]payments.ProductUsagePriceModel
	defaults  Defaults

	adminConfig   Config
	consoleConfig console.Config

	nowFn func() time.Time
}

// NewService creates a new satellite administration service.
func NewService(
	log *zap.Logger,
	consoleDB console.DB,
	history changehistory.DB,
	accountingDB accounting.ProjectAccounting,
	accounting *accounting.Service,
	authorizer *Authorizer,
	accountFreeze *console.AccountFreezeService,
	analytics *analytics.Service,
	buckets *buckets.Service,
	entitlements *entitlements.Service,
	metabaseDB *metabase.DB,
	logger *auditlogger.Logger,
	payments payments.Accounts,
	restKeys restapikeys.Service,
	placement nodeselection.PlacementDefinitions,
	products map[int32]payments.ProductUsagePriceModel,
	defaultMaxBuckets int,
	defaultRateLimit float64,
	adminConfig Config,
	consoleConfig console.Config,
	nowFn func() time.Time,
) *Service {
	return &Service{
		log:           log,
		consoleDB:     consoleDB,
		history:       history,
		restKeys:      restKeys,
		analytics:     analytics,
		accountingDB:  accountingDB,
		accounting:    accounting,
		accountFreeze: accountFreeze,
		authorizer:    authorizer,
		auditLogger:   logger,
		buckets:       buckets,
		entitlements:  entitlements,
		metabase:      metabaseDB,
		payments:      payments,
		placement:     placement,
		products:      products,
		defaults: Defaults{
			MaxBuckets: defaultMaxBuckets,
			RateLimit:  int(defaultRateLimit),
		},
		adminConfig:   adminConfig,
		consoleConfig: consoleConfig,
		nowFn:         nowFn,
	}
}

// StatusInfo contains the name and value of a status.
type StatusInfo struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// SearchResult contains the result of a search for users or projects.
type SearchResult struct {
	// projects are only "searched" by their ID, so only one project is returned.
	Project  *Project     `json:"project"`
	Accounts []AccountMin `json:"accounts"`
}

// SearchUsersOrProjects searches for users and projects matching the given term.
func (s *Service) SearchUsersOrProjects(ctx context.Context, authInfo *AuthInfo, term string) (*SearchResult, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if authInfo == nil || len(authInfo.Groups) == 0 {
		return nil, apiError(http.StatusUnauthorized, errs.New("not authorized"))
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

	if !hasPerm(PermAccountView) && !hasPerm(PermProjectView) {
		return nil, apiError(http.StatusForbidden, errs.New("not authorized"))
	}

	if hasPerm(PermProjectView) {
		if id, err := uuid.FromString(term); err == nil {
			p, apiErr := s.GetProject(ctx, id)
			if apiErr.Err != nil && apiErr.Status != http.StatusNotFound {
				return nil, apiErr
			}
			if p != nil {
				return &SearchResult{Project: p}, api.HTTPError{}
			}
		}
	}
	emptyResult := SearchResult{Accounts: []AccountMin{}}

	if !hasPerm(PermAccountView) {
		return &emptyResult, api.HTTPError{}
	}

	users, apiErr := s.SearchUsers(ctx, term)
	if apiErr.Err != nil {
		return nil, apiErr
	}
	if len(users) == 0 {
		return &emptyResult, api.HTTPError{}
	}

	return &SearchResult{Accounts: users}, api.HTTPError{}
}

// GetChangeHistory retrieves the change history for a specific user, project, and bucket.
func (s *Service) GetChangeHistory(ctx context.Context, exact string, itemType string, id string) ([]changehistory.ChangeLog, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) ([]changehistory.ChangeLog, api.HTTPError) {
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	var changes []changehistory.ChangeLog
	switch changehistory.ItemType(itemType) {
	case changehistory.ItemTypeUser:
		uuID, err := uuid.FromString(id)
		if err != nil {
			return apiError(http.StatusBadRequest, errs.New("invalid user ID"))
		}
		changes, err = s.history.GetChangesByUserID(ctx, uuID, exact == "true")
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
	case changehistory.ItemTypeProject:
		uuID, err := uuid.FromString(id)
		if err != nil {
			return apiError(http.StatusBadRequest, errs.New("invalid project ID"))
		}
		changes, err = s.history.GetChangesByProjectID(ctx, uuID, exact == "true")
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
	case changehistory.ItemTypeBucket:
		changes, err = s.history.GetChangesByBucketName(ctx, id)
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
	default:
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("at least one of userID, projectID, or bucketID must be provided"),
		}
	}

	return changes, api.HTTPError{}
}

// TestSetRoleViewer sets a role to viewer for testing purposes.
func (s *Service) TestSetRoleViewer(role string) {
	s.authorizer.groupsRoles[role] = RoleViewer
}

// TestSetBypassAuth sets whether to bypass authentication. This is only for testing purposes.
func (s *Service) TestSetBypassAuth(bypass bool) {
	s.authorizer.enabled = !bypass
}

// TestSetAllowedHost sets the allowed host for oauth. This is only for testing purposes.
func (s *Service) TestSetAllowedHost(host string) {
	s.authorizer.allowedHost = host
}

// TestSetNowFn sets the function to get the current time. This is only for testing purposes.
func (s *Service) TestSetNowFn(nowFn func() time.Time) {
	s.nowFn = nowFn
}

// TestToggleAuditLogger enables or disables the audit logger for testing purposes.
func (s *Service) TestToggleAuditLogger(enabled bool) {
	s.auditLogger.TestToggleAuditLogger(enabled)
}
