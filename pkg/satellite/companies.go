// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Companies exposes methods to manage Company table in database.
type Companies interface {
	// GetByUserID is a method for querying company from the database by userID
	GetByUserID(ctx context.Context, userID uuid.UUID) (*Company, error)
	// Insert is a method for inserting company into the database
	Insert(ctx context.Context, company *Company) (*Company, error)
	// Delete is a method for deleting company by userID from the database.
	Delete(ctx context.Context, userID uuid.UUID) error
	// Update is a method for updating company entity
	Update(ctx context.Context, company *Company) error
}

// CompanyInfo holds data needed to create/update Company
type CompanyInfo struct {
	Name       string
	Address    string
	Country    string
	City       string
	State      string
	PostalCode string
}

// Company is a database object that describes Company entity
type Company struct {
	//PK and FK on users table
	UserID uuid.UUID

	Name       string
	Address    string
	Country    string
	City       string
	State      string
	PostalCode string

	CreatedAt time.Time
}
