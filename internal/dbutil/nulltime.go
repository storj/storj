// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"database/sql/driver"
	"time"
)

// NullTime time helps convert nil to time.Time
type NullTime struct {
	time.Time
	Valid bool
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}
