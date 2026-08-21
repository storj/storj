// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"time"

	"storj.io/common/uuid"
)

// ErrLicenseRecordExists is error class defining that such license record already exists.
var ErrLicenseRecordExists = Error.New("invoice license record already exists")

// ErrDuplicateLicenseRecord is returned when one Create call is given more than one
// record for the same user, product and prorated rate.
var ErrDuplicateLicenseRecord = Error.New("duplicate invoice license record in batch")

// LicenseRecordsDB is interface for working with invoice license records.
//
// architecture: Database
type LicenseRecordsDB interface {
	// Create creates new invoice license records in the DB. The records must be
	// distinct by user, product and billed days, since that is the identity of a
	// charge; a batch containing two records for the same one is rejected with
	// ErrDuplicateLicenseRecord rather than written in part. Callers aggregate seats
	// per rate before recording them.
	Create(ctx context.Context, records []CreateLicenseRecord, start, end time.Time) error
	// Check checks if an invoice license record for the specified user, product,
	// prorated rate and billing period exists.
	Check(ctx context.Context, userID uuid.UUID, productID int32, billedDays int, start, end time.Time) error
	// ListUnappliedByUser returns the records for a user and billing period that have
	// not been turned into invoice items yet.
	ListUnappliedByUser(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]LicenseRecord, error)
	// Consume marks an invoice license record as applied.
	Consume(ctx context.Context, id uuid.UUID) error
}

// CreateLicenseRecord holds the info needed to create a new invoice license record.
type CreateLicenseRecord struct {
	UserID    uuid.UUID
	ProductID int32
	// Seats is the number of license seats billed at this rate.
	Seats int64
	// BilledDays is how many days of the period the seats are charged for, and
	// DaysInPeriod the length of the period. Together they are the proration.
	BilledDays   int
	DaysInPeriod int
	// UnitAmountCents is the prorated price charged per seat.
	UnitAmountCents int64
}

// LicenseRecord holds a license seat charge for a billing period.
type LicenseRecord struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	ProductID       int32
	Seats           int64
	BilledDays      int
	DaysInPeriod    int
	UnitAmountCents int64
	PeriodStart     time.Time
	PeriodEnd       time.Time
	State           int
}
