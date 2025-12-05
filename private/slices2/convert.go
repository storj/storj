// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package slices2

// Convert converts xs by applying fn to each element.
// If there's an error during conversion, the function
// returns an empty slice and the error.
func Convert[In, Out any](xs []In, fn func(In) (Out, error)) ([]Out, error) {
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

// Map converts xs by applying fn to each element.
func Map[In, Out any](xs []In, fn func(In) Out) []Out {
	rs := make([]Out, len(xs))
	for i := range xs {
		rs[i] = fn(xs[i])
	}
	return rs
}

// ConvertErrs converts xs by applying fn to each element.
// It returns all the successfully converted values and returns the list of
// errors separately.
func ConvertErrs[In, Out any](xs []In, fn func(In) (Out, error)) ([]Out, []error) {
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
