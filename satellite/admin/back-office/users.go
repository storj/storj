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
	PaidTier         bool                      `json:"paidTier"`
	CreatedAt        time.Time                 `json:"createdAt"`
	Status           string                    `json:"status"`
	UserAgent        string                    `json:"userAgent"`
	DefaultPlacement storj.PlacementConstraint `json:"defaultPlacement"`
	Projects         []UserProject             `json:"projects"`
}

// UserProject is project owned by a user with  basic information, usage, and limits.
type UserProject struct {
	ID   uuid.UUID `json:"id"` // This is the public ID
	Name string    `json:"name"`
	ProjectUsageLimits[int64]
}

// GetUserByEmail returns a verified user by its email address.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (*UserAccount, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.consoleDB.Users().GetByEmail(ctx, email)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	projects, err := s.consoleDB.Projects().GetOwn(ctx, user.ID)
	if err != nil {
		return nil, api.HTTPError{
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
			return nil, apiErr
		}

		bandwidthu, storageu, segmentu, apiErr := s.getProjectUsage(ctx, p.ID)
		if apiErr.Err != nil {
			if apiErr.Status == http.StatusNotFound {
				// We return a conflict if a project doesn't exists because it means that the project was
				// deleted between getting the list of projects of the user and retrieving the usage and
				// limits of the project.
				apiErr.Status = http.StatusConflict
			}
			return nil, apiErr
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

	return &UserAccount{
		User: User{
			ID:       user.ID,
			FullName: user.FullName,
			Email:    user.Email,
		},
		PaidTier:         user.PaidTier,
		CreatedAt:        user.CreatedAt,
		Status:           user.Status.String(),
		UserAgent:        string(user.UserAgent),
		DefaultPlacement: user.DefaultPlacement,
		Projects:         usageLimits,
	}, api.HTTPError{}
}
