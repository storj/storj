// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/console"
)

// User holds the user's information.
type User struct {
	ID       uuid.UUID `json:"id"`
	FullName string    `json:"fullName"`
	Email    string    `json:"email"`
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
}

// UserProject is project owned by a user with  basic information, usage, and limits.
type UserProject struct {
	ID   uuid.UUID `json:"id"` // This is the public ID
	Name string    `json:"name"`
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
		Projects:         usageLimits,
		ProjectLimit:     user.ProjectLimit,
		StorageLimit:     user.ProjectStorageLimit,
		BandwidthLimit:   user.ProjectBandwidthLimit,
		SegmentLimit:     user.ProjectSegmentLimit,
		FreezeStatus:     freezeStatus,
		TrialExpiration:  user.TrialExpiration,
	}, api.HTTPError{}
}

// GetUserByEmail returns information about a user by their email address.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.consoleDB.Users().GetByEmail(ctx, email)
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
		Status:           user.Status.Info(),
		UserAgent:        string(user.UserAgent),
		DefaultPlacement: user.DefaultPlacement,
		Projects:         usageLimits,
		ProjectLimit:     user.ProjectLimit,
		StorageLimit:     user.ProjectStorageLimit,
		BandwidthLimit:   user.ProjectBandwidthLimit,
		SegmentLimit:     user.ProjectSegmentLimit,
		FreezeStatus:     freezeStatus,
		TrialExpiration:  user.TrialExpiration,
	}, api.HTTPError{}
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
			ID:   p.PublicID,
			Name: p.Name,
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
