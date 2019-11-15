// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Projects exposes methods to manage Project table in database.
//
// architecture: Database
type Projects interface {
	// GetAll is a method for querying all projects from the database.
	GetAll(ctx context.Context) ([]Project, error)
	// GetCreatedBefore retrieves all projects created before provided date.
	GetCreatedBefore(ctx context.Context, before time.Time) ([]Project, error)
	// GetByUserID is a method for querying all projects from the database by userID.
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// GetOwn is a method for querying all projects created by current user from the database.
	GetOwn(ctx context.Context, userID uuid.UUID) (_ []Project, err error)
	// Get is a method for querying project from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*Project, error)
	// Insert is a method for inserting project into the database.
	Insert(ctx context.Context, project *Project) (*Project, error)
	// Delete is a method for deleting project by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating project entity.
	Update(ctx context.Context, project *Project) error
	// List returns paginated projects, created before provided timestamp.
	List(ctx context.Context, offset int64, limit int, before time.Time) (ProjectsPage, error)
}

// Project is a database object that describes Project entity
type Project struct {
	ID uuid.UUID `json:"id"`

	Name        string    `json:"name"`
	Description string    `json:"description"`
	UsageLimit  int64     `json:"usageLimit"`
	PartnerID   uuid.UUID `json:"partnerId"`
	OwnerID     uuid.UUID `json:"ownerId"`

	CreatedAt time.Time `json:"createdAt"`
}

// ProjectInfo holds data needed to create/update Project
type ProjectInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`

	CreatedAt time.Time `json:"createdAt"`
}

// ProjectsPage returns paginated projects,
// providing next offset if there are more projects
// to retrieve.
type ProjectsPage struct {
	Projects   []Project
	Next       bool
	NextOffset int64
}
