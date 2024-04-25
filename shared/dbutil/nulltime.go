// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/zeebo/errs"
)

const (
	sqliteTimeLayout           = "2006-01-02 15:04:05-07:00"
	sqliteTimeLayoutNoTimeZone = "2006-01-02 15:04:05"
	sqliteTimeLayoutDate       = "2006-01-02"
)

// ErrNullTime defines error class for NullTime.
var ErrNullTime = errs.Class("null time")

// NullTime time helps convert nil to time.Time.
type NullTime struct {
	time.Time
	Valid bool
}

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value interface{}) error {
	nt.Time, nt.Valid = time.Time{}, false

	if value == nil {
		return nil
	}

	switch v := value.(type) {
	// Postgres could return for lagged time values.
	case time.Time:
		nt.Time, nt.Valid = v, true

		// Database could return for nullable time values.
	case sql.NullTime:
		nt.Time, nt.Valid = v.Time, v.Valid

		// Sqlite may return this.
	case string:
		parsed, err := parseSqliteTimeString(v)
		if err != nil {
			return ErrNullTime.Wrap(err)
		}
		nt.Time, nt.Valid = parsed, true

		// Sqlite may return this.
	case []byte:
		parsed, err := parseSqliteTimeString(string(v))
		if err != nil {
			return ErrNullTime.Wrap(err)
		}
		nt.Time, nt.Valid = parsed, true

	default:
		return ErrNullTime.New("scan received unsupported value %T", value)
	}

	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if !nt.Valid {
		return nil, nil
	}
	return nt.Time, nil
}

// parseSqliteTimeString parses sqlite times string.
// It tries to process value as string with timezone first,
// then fallback to parsing as string without timezone and
// finally to parsing value as date.
func parseSqliteTimeString(val string) (time.Time, error) {
	var times time.Time
	var err error

	times, err = time.Parse(sqliteTimeLayout, val)
	if err == nil {
		return times, nil
	}

	times, err = time.Parse(sqliteTimeLayoutNoTimeZone, val)
	if err == nil {
		return times, nil
	}

	return time.Parse(sqliteTimeLayoutDate, val)
}
