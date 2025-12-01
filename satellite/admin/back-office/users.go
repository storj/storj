// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/admin/back-office/auditlogger"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
)

// User holds the user's information.
type User struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"fullName"`
	Email    string    `json:"email"`
}

// AccountMin holds minimal information about a user's account.
type AccountMin struct {
	ID        uuid.UUID              `json:"id"`
	FullName  string                 `json:"fullName"`
	Email     string                 `json:"email"`
	Kind      console.KindInfo       `json:"kind"`
	Status    console.UserStatusInfo `json:"status"`
	CreatedAt time.Time              `json:"createdAt"`
}

// UserAccount holds information about a user's account.
type UserAccount struct {
	User
	Kind             console.KindInfo          `json:"kind"`
	CreatedAt        time.Time                 `json:"createdAt"`
	UpgradeTime      *time.Time                `json:"upgradeTime"`
	Status           console.UserStatusInfo    `json:"status"`
	UserAgent        string                    `json:"userAgent"`
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`
	Projects         []UserProject             `json:"projects"`
	ProjectLimit     int                       `json:"projectLimit"`
	StorageLimit     int64                     `json:"storageLimit"`
	BandwidthLimit   int64                     `json:"bandwidthLimit"`
	SegmentLimit     int64                     `json:"segmentLimit"`
	FreezeStatus     *FreezeEventType          `json:"freezeStatus"`
	TrialExpiration  *time.Time                `json:"trialExpiration"`
	MFAEnabled       bool                      `json:"mfaEnabled"`
}

// UpdateUserRequest represents a request to update a user.
type UpdateUserRequest struct {
	Email           *string             `json:"email"`
	Name            *string             `json:"name"`
	Kind            *console.UserKind   `json:"kind"`
	Status          *console.UserStatus `json:"status"`
	TrialExpiration *string             `json:"trialExpiration"` // in RFC3339 format, empty string to clear
	UserAgent       *string             `json:"userAgent"`
	ProjectLimit    *int                `json:"projectLimit"`
	StorageLimit    *int64              `json:"storageLimit"`
	BandwidthLimit  *int64              `json:"bandwidthLimit"`
	SegmentLimit    *int64              `json:"segmentLimit"`

	Reason string `json:"reason"` // reason for audit log
}

func (r *UpdateUserRequest) parseTrialExpiration() (**time.Time, error) {
	if r.TrialExpiration == nil {
		return nil, nil
	}
	t := new(*time.Time)
	if *r.TrialExpiration == "" {
		*t = nil
		return t, nil
	}
	parsed, err := time.Parse(time.RFC3339, *r.TrialExpiration)
	if err != nil {
		return nil, err
	}
	*t = &parsed

	return t, nil
}

// DisableUserRequest represents a request to disable a user.
type DisableUserRequest struct {
	SetPendingDeletion bool   `json:"setPendingDeletion"`
	Reason             string `json:"reason"` // reason for audit log
}

// CreateRestKeyRequest represents a request to create rest key.
type CreateRestKeyRequest struct {
	Expiration time.Time `json:"expiration"`

	Reason string `json:"reason"` // reason for audit log
}

// ToggleMfaRequest represents a request to (enable or) disable MFA for a user.
type ToggleMfaRequest struct {
	Reason string `json:"reason"` // reason for audit log
}

// UserProject is project owned by a user with  basic information, usage, and limits.
type UserProject struct {
	ID       uuid.UUID `json:"-"`
	PublicID uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	Active   bool      `json:"active"`
	ProjectUsageLimits[int64]
}

// GetUserKinds returns the list of available user kinds.
func (s *Service) GetUserKinds(ctx context.Context) ([]console.KindInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	kinds := make([]console.KindInfo, len(console.UserKinds))
	for i, k := range console.UserKinds {
		kinds[i] = k.Info()
	}
	return kinds, api.HTTPError{}
}

// GetUserStatuses returns the list of available user statuses.
func (s *Service) GetUserStatuses(ctx context.Context) ([]console.UserStatusInfo, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	statuses := make([]console.UserStatusInfo, len(console.UserStatuses))
	for i, us := range console.UserStatuses {
		statuses[i] = us.Info()
	}
	return statuses, api.HTTPError{}
}

// SearchUsers searches for users by a search term in their name, email, customer ID or by their ID.
func (s *Service) SearchUsers(ctx context.Context, term string) ([]AccountMin, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if len(term) < 3 {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("search term must be at least 3 characters"),
		}
	}

	// check if the term is a valid UUID
	if id, err := uuid.FromString(term); err == nil {
		user, err := s.consoleDB.Users().Get(ctx, id)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
		if user != nil {
			return []AccountMin{{
				ID:        user.ID,
				FullName:  user.FullName,
				Email:     user.Email,
				Kind:      user.Kind.Info(),
				Status:    user.Status.Info(),
				CreatedAt: user.CreatedAt,
			}}, api.HTTPError{}
		}
		return make([]AccountMin, 0), api.HTTPError{}
	}

	// check whether the term is a stripe customer ID
	if strings.HasPrefix(term, "cus_") {
		user, err := s.consoleDB.Users().GetByCustomerID(ctx, term)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
		if user != nil {
			return []AccountMin{{
				ID:        user.ID,
				FullName:  user.FullName,
				Email:     user.Email,
				Kind:      user.Kind.Info(),
				Status:    user.Status.Info(),
				CreatedAt: user.CreatedAt,
			}}, api.HTTPError{}
		}
		return make([]AccountMin, 0), api.HTTPError{}
	}

	// search by name or email
	uPage, err := s.consoleDB.Users().Search(ctx, term)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	users := make([]AccountMin, 0, len(uPage))
	for _, u := range uPage {
		users = append(users, AccountMin{
			ID:        u.ID,
			FullName:  u.FullName,
			Email:     u.Email,
			Kind:      u.Kind.Info(),
			Status:    u.Status.Info(),
			CreatedAt: u.CreatedAt,
		})
	}

	return users, api.HTTPError{}
}

// GetUser returns information about a user by their ID.
func (s *Service) GetUser(ctx context.Context, userID uuid.UUID) (*UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	return s.getUserAccount(ctx, user)
}

// GetUserByEmail returns information about a user by their email address.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.consoleDB.Users().GetByEmailAndTenant(ctx, email, nil)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	return s.getUserAccount(ctx, user)
}

func (s *Service) getUsageLimitsAndFreezes(ctx context.Context, userID uuid.UUID) ([]UserProject, *FreezeEventType, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	projects, err := s.consoleDB.Projects().GetOwn(ctx, userID)
	if err != nil {
		return nil, nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	usageLimits := make([]UserProject, 0, len(projects))
	for _, p := range projects {
		bandwidthl, storagel, segmentl, apiErr := s.getProjectLimits(ctx, p.ID)
		if apiErr.Err != nil {
			if apiErr.Status == http.StatusNotFound {
				// We return a conflict if a project doesn't exists because it means that the project was
				// deleted between getting the list of projects of the user and retrieving the usage and
				// limits of the project.
				apiErr.Status = http.StatusConflict
			}
			return nil, nil, apiErr
		}

		bandwidthu, storageu, segmentu, apiErr := s.getProjectUsage(ctx, p.ID)
		if apiErr.Err != nil {
			if apiErr.Status == http.StatusNotFound {
				// We return a conflict if a project doesn't exists because it means that the project was
				// deleted between getting the list of projects of the user and retrieving the usage and
				// limits of the project.
				apiErr.Status = http.StatusConflict
			}
			return nil, nil, apiErr
		}

		usageLimits = append(usageLimits, UserProject{
			ID:       p.ID,
			PublicID: p.PublicID,
			Name:     p.Name,
			Active:   p.Status != nil && *p.Status == console.ProjectActive,
			ProjectUsageLimits: ProjectUsageLimits[int64]{
				BandwidthLimit: bandwidthl,
				BandwidthUsed:  bandwidthu,
				StorageLimit:   storagel,
				StorageUsed:    storageu,
				SegmentLimit:   segmentl,
				SegmentUsed:    segmentu,
			},
		})
	}

	freezes, err := s.accountFreeze.GetAll(ctx, userID)
	if err != nil {
		return nil, nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	var freezeEvent *console.AccountFreezeEvent
	if freezes.BillingFreeze != nil {
		freezeEvent = freezes.BillingFreeze
	} else if freezes.LegalFreeze != nil {
		freezeEvent = freezes.LegalFreeze
	} else if freezes.ViolationFreeze != nil {
		freezeEvent = freezes.ViolationFreeze
	} else if freezes.TrialExpirationFreeze != nil {
		freezeEvent = freezes.TrialExpirationFreeze
	} else if freezes.BotFreeze != nil {
		freezeEvent = freezes.BotFreeze
	} else if freezes.DelayedBotFreeze != nil {
		freezeEvent = freezes.DelayedBotFreeze
	} else if freezes.BillingWarning != nil {
		freezeEvent = freezes.BillingWarning
	}
	var freezeStatus *FreezeEventType
	if freezeEvent != nil {
		freezeStatus = &FreezeEventType{
			Name:  freezeEvent.Type.String(),
			Value: freezeEvent.Type,
		}
	}

	return usageLimits, freezeStatus, api.HTTPError{}
}

// UpdateUser updates a user's information.
// Limit updates will cascade to all projects owned by the user.
// Email updates will also update the email in the payment and analytics systems.
func (s *Service) UpdateUser(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request UpdateUserRequest) (*UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		e := Error.Wrap(err)
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			e = Error.New("user not found")
		}
		return nil, api.HTTPError{Status: status, Err: e}
	}

	apiErr := s.validateUpdateRequest(ctx, authInfo, user, request)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	beforeState, apiErr := s.getUserAccount(ctx, user)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	var userAgent []byte
	if request.UserAgent != nil {
		userAgent = []byte(*request.UserAgent)
	}

	var upgradeTime *time.Time
	trialExpiration, _ := request.parseTrialExpiration()
	if request.Kind != nil && *request.Kind != user.Kind {
		now := s.nowFn()
		if *request.Kind == console.PaidUser {
			upgradeTime = &now
		}
		if *request.Kind == console.FreeUser {
			if request.TrialExpiration == nil {
				if s.consoleConfig.FreeTrialDuration != 0 {
					expiration := now.Add(s.consoleConfig.FreeTrialDuration)
					trialExpiration = new(*time.Time)
					*trialExpiration = &expiration
				}
			}
		} else if *request.Kind == console.MemberUser {
			if beforeState != nil {
				for _, p := range beforeState.Projects {
					if !p.Active {
						continue
					}
					return nil, api.HTTPError{
						Status: http.StatusForbidden,
						Err:    Error.New("cannot change to member user while having active projects"),
					}
				}
			}

			trialExpiration = new(*time.Time)
		} else {
			trialExpiration = new(*time.Time)
			ptrInt := func(i int64) *int64 { return &i }
			limits := map[string]map[console.UserKind]any{
				"project": {
					console.PaidUser: &s.consoleConfig.UsageLimits.Project.Paid,
					console.NFRUser:  &s.consoleConfig.UsageLimits.Project.Nfr,
				},
				"storage": {
					console.PaidUser: ptrInt(s.consoleConfig.UsageLimits.Storage.Paid.Int64()),
					console.NFRUser:  ptrInt(s.consoleConfig.UsageLimits.Storage.Nfr.Int64()),
				},
				"bandwidth": {
					console.PaidUser: ptrInt(s.consoleConfig.UsageLimits.Bandwidth.Paid.Int64()),
					console.NFRUser:  ptrInt(s.consoleConfig.UsageLimits.Bandwidth.Nfr.Int64()),
				},
				"segment": {
					console.PaidUser: &s.consoleConfig.UsageLimits.Segment.Paid,
					console.NFRUser:  &s.consoleConfig.UsageLimits.Segment.Nfr,
				},
			}

			// admin set limits take precedence over kind defaults
			if request.ProjectLimit == nil {
				request.ProjectLimit = limits["project"][*request.Kind].(*int)
			}
			if request.StorageLimit == nil {
				request.StorageLimit = limits["storage"][*request.Kind].(*int64)
			}
			if request.BandwidthLimit == nil {
				request.BandwidthLimit = limits["bandwidth"][*request.Kind].(*int64)
			}
			if request.SegmentLimit == nil {
				request.SegmentLimit = limits["segment"][*request.Kind].(*int64)
			}
		}
	}

	var projectChangeEvents []auditlogger.Event
	err = s.consoleDB.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		usersDB := tx.Users()
		err = usersDB.Update(ctx, userID, console.UpdateUserRequest{
			Email:                 request.Email,
			FullName:              request.Name,
			Kind:                  request.Kind,
			UpgradeTime:           upgradeTime,
			TrialExpiration:       trialExpiration,
			Status:                request.Status,
			UserAgent:             userAgent,
			ProjectLimit:          request.ProjectLimit,
			ProjectStorageLimit:   request.StorageLimit,
			ProjectBandwidthLimit: request.BandwidthLimit,
			ProjectSegmentLimit:   request.SegmentLimit,
		})
		if err != nil {
			return err
		}

		if request.Email != nil && *request.Email != user.Email {
			// update email in analytics and payment system
			s.analytics.ChangeContactEmail(user.ID, user.Email, *request.Email)
			cusID, err := usersDB.GetCustomerID(ctx, user.ID)
			if err != nil {
				return err
			}

			if err = s.payments.ChangeCustomerEmail(ctx, user.ID, cusID, *request.Email); err != nil {
				s.log.Error("Failed to update customer email on stripe", zap.Stringer("userId", user.ID), zap.Error(err))
				return err
			}
		}

		if request.StorageLimit == nil && request.BandwidthLimit == nil && request.SegmentLimit == nil {
			return nil
		}

		projectsDB := tx.Projects()
		for _, p := range beforeState.Projects {
			after := p
			var toUpdate []console.Limit
			if request.StorageLimit != nil {
				after.StorageLimit = *request.StorageLimit
				toUpdate = append(toUpdate, console.Limit{
					Kind: console.StorageLimit, Value: request.StorageLimit,
				})
			}
			if request.BandwidthLimit != nil {
				after.BandwidthLimit = *request.BandwidthLimit
				toUpdate = append(toUpdate, console.Limit{
					Kind: console.BandwidthLimit, Value: request.BandwidthLimit,
				})
			}
			if request.SegmentLimit != nil {
				after.SegmentLimit = *request.SegmentLimit
				toUpdate = append(toUpdate, console.Limit{
					Kind: console.SegmentLimit, Value: request.SegmentLimit,
				})
			}
			if err = projectsDB.UpdateLimitsGeneric(ctx, p.ID, toUpdate); err != nil {
				return err
			}
			if p == after {
				continue
			}
			projectChangeEvents = append(projectChangeEvents, auditlogger.Event{
				UserID:     userID,
				ProjectID:  &p.PublicID,
				Action:     "update_project",
				AdminEmail: authInfo.Email,
				ItemType:   changehistory.ItemTypeProject,
				Reason:     request.Reason,
				Before:     p,
				After:      after,
				Timestamp:  s.nowFn(),
			})
		}

		return nil
	})
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	updatedUser, apiErr := s.GetUser(ctx, userID)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	for _, event := range projectChangeEvents {
		s.auditLogger.EnqueueChangeEvent(event)
	}
	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "update_user",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     beforeState,
		After:      updatedUser,
		Timestamp:  s.nowFn(),
	})

	return updatedUser, api.HTTPError{}
}

func (s *Service) validateUpdateRequest(ctx context.Context, authInfo *AuthInfo, user *console.User, request UpdateUserRequest) api.HTTPError {
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
	hasPerm := func(perm ...Permission) bool {
		for _, g := range groups {
			if s.authorizer.HasPermissions(g, perm...) {
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
		if !hasPerm(PermAccountChangeStatus) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change user status"))
		}
		if *request.Status == console.PendingDeletion {
			// this is because setting to pending deletion may lead to data deletion by a chore
			return apiError(http.StatusForbidden, errs.New("not authorized to set user status to pending deletion"))
		}
		for _, us := range console.UserStatuses {
			if *request.Status == us {
				valid = true
				break
			}
		}
		if !valid {
			errGroup = append(errGroup, errs.New("invalid user status %d", *request.Status))
		}
	}

	if request.Kind != nil {
		if !hasPerm(PermAccountChangeKind) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change user kind"))
		}
		for _, k := range console.UserKinds {
			if *request.Kind == k {
				valid = true
				break
			}
		}
		if !valid {
			errGroup = append(errGroup, errs.New("invalid user kind %d", *request.Kind))
		}
	}

	trialExpiration, err := request.parseTrialExpiration()
	if err != nil {
		errGroup = append(errGroup, errs.New("invalid trial expiration format, must be RFC3339"))
	} else if trialExpiration != nil {
		if !hasPerm(PermAccountChangeKind) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change trial expiration"))
		}
		if *trialExpiration != nil {
			if (*trialExpiration).Before(s.nowFn()) {
				errGroup = append(errGroup, errs.New("trial expiration must be in the future"))
			}
			if request.Kind != nil && *request.Kind != console.FreeUser {
				errGroup = append(errGroup, errs.New("trial expiration can only be set for free users"))
			} else if request.Kind == nil && user.Kind != console.FreeUser {
				errGroup = append(errGroup, errs.New("trial expiration can only be set for free users"))
			}
		}
	}

	if request.Name != nil {
		if !hasPerm(PermAccountChangeName) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change user name"))
		}
		if *request.Name == "" {
			errGroup = append(errGroup, errs.New("name cannot be empty"))
		}
	}

	if request.UserAgent != nil && !hasPerm(PermAccountSetUserAgent) {
		return apiError(http.StatusForbidden, errs.New("not authorized to set user agent"))
	}

	if request.ProjectLimit != nil || request.StorageLimit != nil || request.BandwidthLimit != nil || request.SegmentLimit != nil {
		if !hasPerm(PermAccountChangeLimits) {
			return apiError(http.StatusForbidden, errs.New("not authorized to change user limits"))
		}
		if request.ProjectLimit != nil && *request.ProjectLimit < 0 {
			errGroup = append(errGroup, errs.New("project limit cannot be negative"))
		}
		if request.StorageLimit != nil && *request.StorageLimit < 0 {
			errGroup = append(errGroup, errs.New("storage limit cannot be negative"))
		}
		if request.BandwidthLimit != nil && *request.BandwidthLimit < 0 {
			errGroup = append(errGroup, errs.New("bandwidth limit cannot be negative"))
		}
		if request.SegmentLimit != nil && *request.SegmentLimit < 0 {
			errGroup = append(errGroup, errs.New("segment limit cannot be negative"))
		}
	}
	if errGroup != nil {
		return apiError(http.StatusBadRequest, errGroup.Err())
	}

	if request.Email != nil && *request.Email != user.Email {
		if !utils.ValidateEmail(*request.Email) {
			return apiError(http.StatusBadRequest, errs.New("invalid email format"))
		}
		existing, err := s.consoleDB.Users().GetByEmailAndTenant(ctx, *request.Email, user.TenantID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return apiError(http.StatusInternalServerError, err)
		}
		if existing != nil {
			return apiError(http.StatusConflict, errs.New("email %q is already in use", *request.Email))
		}
	}

	return api.HTTPError{}
}

func (s *Service) getUserAccount(ctx context.Context, user *console.User) (*UserAccount, api.HTTPError) {
	usageLimits, freezeStatus, apiErr := s.getUsageLimitsAndFreezes(ctx, user.ID)
	if apiErr.Err != nil {
		return nil, apiErr
	}

	return &UserAccount{
		User: User{
			ID:       user.ID,
			FullName: user.FullName,
			Email:    user.Email,
		},
		Kind:             user.Kind.Info(),
		CreatedAt:        user.CreatedAt,
		UpgradeTime:      user.UpgradeTime,
		Status:           user.Status.Info(),
		UserAgent:        string(user.UserAgent),
		DefaultPlacement: user.DefaultPlacement,
		ProjectLimit:     user.ProjectLimit,
		StorageLimit:     user.ProjectStorageLimit,
		BandwidthLimit:   user.ProjectBandwidthLimit,
		SegmentLimit:     user.ProjectSegmentLimit,
		TrialExpiration:  user.TrialExpiration,
		MFAEnabled:       user.MFAEnabled,
		Projects:         usageLimits,
		FreezeStatus:     freezeStatus,
	}, api.HTTPError{}
}

// DisableUser deactivates a user if they have no active projects or unpaid invoices.
func (s *Service) DisableUser(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request DisableUserRequest) (*UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) (*UserAccount, api.HTTPError) {
		return nil, api.HTTPError{
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
		if !hasPerm(PermAccountDeleteWithData, PermAccountMarkPendingDeletion) {
			return apiError(http.StatusForbidden, errs.New("not authorized to mark user pending deletion"))
		}
		if !s.adminConfig.PendingDeleteUserCleanupEnabled {
			return apiError(http.StatusConflict, errs.New("marking user as pending deletion is not enabled"))
		}
	} else {
		if !hasPerm(PermAccountDeleteNoData) {
			return apiError(http.StatusForbidden, errs.New("not authorized to delete user"))
		}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("user not found")
		}
		return apiError(status, err)
	}

	if !request.SetPendingDeletion {
		projects, err := s.consoleDB.Projects().GetOwnActive(ctx, user.ID)
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}
		if len(projects) > 0 {
			return apiError(http.StatusConflict, errs.New("user has active projects"))
		}
	}

	// ensure no unpaid invoices exist.
	hasUnpaid, err := s.hasUnpaidInvoices(ctx, user.ID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}
	if hasUnpaid {
		return apiError(http.StatusConflict, errs.New("user has unpaid invoices"))
	}

	auditLog := func(action string, before, after console.User) {
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     userID,
			Action:     action,
			AdminEmail: authInfo.Email,
			ItemType:   changehistory.ItemTypeUser,
			Reason:     request.Reason,
			Before:     before,
			After:      after,
			Timestamp:  s.nowFn(),
		})
	}

	status := console.Deleted
	if request.SetPendingDeletion {
		status = console.PendingDeletion
		err = s.consoleDB.Users().Update(ctx, user.ID, console.UpdateUserRequest{Status: &status})
		if err != nil {
			return apiError(http.StatusInternalServerError, err)
		}

		after := *user
		after.Status = status

		auditLog("mark_user_pending_deletion", *user, after)

		return s.getUserAccount(ctx, &after)
	}

	emptyName := ""
	emptyNamePtr := &emptyName
	deactivatedEmail := fmt.Sprintf("deactivated+%s@storj.io", user.ID.String())
	var externalID *string // nil - no external ID.

	var afterState *console.User
	err = s.consoleDB.WithTx(ctx, func(ctx context.Context, tx console.DBTx) error {
		err = tx.Users().Update(ctx, user.ID, console.UpdateUserRequest{
			FullName:   &emptyName,
			ShortName:  &emptyNamePtr,
			Email:      &deactivatedEmail,
			Status:     &status,
			ExternalID: &externalID,
		})
		if err != nil {
			return err
		}

		afterState, err = tx.Users().Get(ctx, user.ID)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	err = s.payments.CreditCards().RemoveAll(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to remove credit cards for deleted user", zap.Stringer("userId", user.ID), zap.Error(err))
	}

	auditLog("disable_user", *user, *afterState)

	return s.getUserAccount(ctx, afterState)
}

func (s *Service) hasUnpaidInvoices(ctx context.Context, userID uuid.UUID) (_ bool, err error) {
	defer mon.Task()(&ctx)(&err)

	invoices, err := s.payments.Invoices().List(ctx, userID)
	if err != nil {
		return false, err
	}
	if len(invoices) > 0 {
		for _, invoice := range invoices {
			if invoice.Status == "draft" || invoice.Status == "open" {
				return true, nil
			}
		}
	}

	return s.payments.Invoices().CheckPendingItems(ctx, userID)
}

// ToggleMFA toggles MFA for a user. Only disabling is supported by admin.
func (s *Service) ToggleMFA(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request ToggleMfaRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	if authInfo == nil {
		return api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.New("not authorized"),
		}
	}

	if request.Reason == "" {
		return api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.New("reason is required"),
		}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	disabledMFA := false
	mfaSecretKeyPtr := new(string)
	var mfaRecoveryCodes []string

	err = s.consoleDB.Users().Update(ctx, user.ID, console.UpdateUserRequest{
		MFAEnabled:       &disabledMFA,
		MFASecretKey:     &mfaSecretKeyPtr,
		MFARecoveryCodes: &mfaRecoveryCodes,
	})
	if err != nil {
		return api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	afterState, err := s.consoleDB.Users().Get(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to retrieve user after toggling MFA", zap.Stringer("userId", user.ID), zap.Error(err))
	} else {
		s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
			UserID:     userID,
			Action:     "toggle_mfa",
			AdminEmail: authInfo.Email,
			ItemType:   changehistory.ItemTypeUser,
			Reason:     request.Reason,
			Before:     user,
			After:      afterState,
			Timestamp:  s.nowFn(),
		})
	}

	return api.HTTPError{}
}

// CreateRestKey creates a new REST API key for a user.
func (s *Service) CreateRestKey(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request CreateRestKeyRequest) (*string, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	var validationErrs errs.Group
	if request.Reason == "" {
		validationErrs = append(validationErrs, errs.New("reason is required"))
	}

	if request.Expiration.IsZero() {
		validationErrs = append(validationErrs, errs.New("expiration is required"))
	}

	expiration := time.Until(request.Expiration)
	if expiration < 0 {
		validationErrs = append(validationErrs, errs.New("expiration must be in the future"))
	}
	if validationErrs != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.Wrap(validationErrs.Err()),
		}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	apiKey, _, err := s.restKeys.CreateNoAuth(ctx, user.ID, &expiration)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "create_rest_key",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     nil,
		After:      apiKey[:5] + "*****",
		Timestamp:  s.nowFn(),
	})

	return &apiKey, api.HTTPError{}
}

// TestToggleAbbreviatedUserDelete is a test helper to toggle abbreviated user deletion.
func (s *Service) TestToggleAbbreviatedUserDelete(enabled bool) {
	s.adminConfig.PendingDeleteUserCleanupEnabled = enabled
}
