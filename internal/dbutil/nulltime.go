// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"database/sql/driver"
	"time"
)

const (
	sqliteTimeLayout = "2006-01-02 15:04:05-07:00"
)

// NullTime time helps convert nil to time.Time
type NullTime struct {
	time.Time
	Valid bool
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	date, ok := value.([]byte)
	if !ok {
		return nil
	}

	times, err := time.Parse(sqliteTimeLayout, string(date))
	if err != nil {
		return nil
	}

	nt.Time, nt.Valid = times, true
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}
