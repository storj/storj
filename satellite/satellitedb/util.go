// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/private/tagsql"
)

// withRows ensures that rows get properly closed after the callback finishes.
func withRows(rows tagsql.Rows, err error) func(func(tagsql.Rows) error) error {
	return func(callback func(tagsql.Rows) error) error {
		if err != nil {
			return err
		}
		err := callback(rows)
		return errs.Combine(rows.Err(), rows.Close(), err)
	}
}

// convertSlice converts xs by applying fn to each element.
// If there's an error during conversion, the function
// returns an empty slice and the error.
func convertSlice[In, Out any](xs []In, fn func(In) (Out, error)) ([]Out, error) {
	rs := make([]Out, len(xs))
	for i := range xs {
		var err error
		rs[i], err = fn(xs[i])
		if err != nil {
			return nil, err
		}
	}
	return rs, nil
}

// convertSliceNoError converts xs by applying fn to each element.
func convertSliceNoError[In, Out any](xs []In, fn func(In) Out) []Out {
	rs := make([]Out, len(xs))
	for i := range xs {
		rs[i] = fn(xs[i])
	}
	return rs
}

// convertSliceWithErrors converts xs by applying fn to each element.
// It returns all the successfully converted values and returns the list of
// errors separately.
func convertSliceWithErrors[In, Out any](xs []In, fn func(In) (Out, error)) ([]Out, []error) {
	var errs []error
	rs := make([]Out, 0, len(xs))
	for i := range xs {
		r, err := fn(xs[i])
		if err != nil {
			errs = append(errs, err)
			continue
		}
		rs = append(rs, r)
	}
	return rs, errs
}

// NullUUID represents a nullable uuid.UUID that can be used as an SQL scan destination.
type NullUUID struct {
	UUID  uuid.UUID
	Valid bool
}

// Scan implements the sql.Scanner interface.
// It scans a value from the database into the NullUUID.
func (nu *NullUUID) Scan(value interface{}) error {
	if value == nil {
		nu.UUID = uuid.UUID{}
		nu.Valid = false
		return nil
	}
	valueBytes, ok := value.([]byte)
	if !ok {
		return errs.New("invalid UUID type")
	}
	copy(nu.UUID[:], valueBytes)
	nu.Valid = true
	return nil
}
