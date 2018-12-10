// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Projects exposes methods to manage Project table in database.
type Projects interface {
	// GetAll is a method for querying all projects from the database.
	GetAll(ctx context.Context) ([]Project, error)
	// GetByOwnerID is a method for querying projects from the database by ownerID.
	GetByOwnerID(ctx context.Context, ownerID uuid.UUID) ([]Project, error)
	// GetByUserID is a method for querying all projects from the database by userID.
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Project, error)
	// Get is a method for querying project from the database by id.
	Get(ctx context.Context, id uuid.UUID) (*Project, error)
	// Insert is a method for inserting project into the database.
	Insert(ctx context.Context, project *Project) (*Project, error)
	// Delete is a method for deleting project by Id from the database.
	Delete(ctx context.Context, id uuid.UUID) error
	// Update is a method for updating project entity.
	Update(ctx context.Context, project *Project) error
}

// Project is a database object that describes Project entity
type Project struct {
	ID uuid.UUID `json:"id"`
	// FK on Users table. ID of project creator.
	OwnerID *uuid.UUID `json:"ownerId"`

	Name        string `json:"name"`
	CompanyName string `json:"companyName"`
	Description string `json:"description"`
	// stores last accepted version of terms of use.
	TermsAccepted int `json:"termsAccepted"`

	CreatedAt time.Time `json:"createdAt"`
}

// ProjectInfo holds data needed to create/update Project
type ProjectInfo struct {
	Name        string `json:"name"`
	CompanyName string `json:"companyName"`
	Description string `json:"description"`
	// Indicates if user accepted Terms & Conditions during project creation on UI
	IsTermsAccepted bool `json:"isTermsAccepted"`

	CreatedAt time.Time `json:"createdAt"`
}
