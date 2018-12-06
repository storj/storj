// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"bytes"
	"encoding/gob"
	"net/url"
	"strings"
	"time"
)

// GetBytes transforms an empty interface type into a byte slice
func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseURL extracts database parameters from a string as a URL
//   bolt://storj.db
//   bolt://C:\storj.db
//   redis://hostname
func ParseURL(s string) (*url.URL, error) {
	if strings.HasPrefix(s, "bolt://") {
		return &url.URL{
			Scheme: "bolt",
			Path:   strings.TrimPrefix(s, "bolt://"),
		}, nil
	}
	if strings.HasPrefix(s, "sqlite3://") {
		return &url.URL{
			Scheme: "sqlite3",
			Path:   strings.TrimPrefix(s, "sqlite3://"),
		}, nil
	}
	if strings.HasPrefix(s, "postgres://") {
		return &url.URL{
			Scheme: "postgres",
			Path:   s,
		}, nil
	}
	return url.Parse(s)
}

// CombineErrors combines multiple errors to a single error
func CombineErrors(errs ...error) error {
	// avoid some allocations
	nonnil := 0
	for _, err := range errs {
		if err != nil {
			nonnil += 1
		}
	}
	errlist := make(ErrorGroup, 0, nonnil)

	for _, err := range errs {
		errlist.Add(err)
	}
	return errlist.Finish()
}

type combinedError []error

func (errs combinedError) Cause() error {
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (errs combinedError) Error() string {
	if len(errs) > 0 {
		limit := 5
		if len(errs) < limit {
			limit = len(errs)
		}
		allErrors := errs[0].Error()
		for _, err := range errs[1:limit] {
			allErrors += "\n" + err.Error()
		}
		return allErrors
	}
	return ""
}

// ErrorGroup contains a set of non-nil errors
type ErrorGroup []error

// Add adds an error to the ErrorGroup if it is non-nil
func (e *ErrorGroup) Add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

// Finish returns nil if there were no non-nil errors, the first error if there
// was only one non-nil error, or the result of CombineErrors if there was more
// than one non-nil error.
func (e ErrorGroup) Finish() error {
	if len(e) == 0 {
		return nil
	}
	if len(e) == 1 {
		return e[0]
	}
	return combinedError(e)
}

// CollectErrors returns first error from channel and all errors that happen within duration
func CollectErrors(errch chan error, duration time.Duration) error {
	errch = discardNil(errch)
	errs := []error{<-errch}
	timeout := time.After(duration)
	for {
		select {
		case err := <-errch:
			errs = append(errs, err)
		case <-timeout:
			return CombineErrors(errs...)
		}
	}
}

// discard nil errors that are returned from services
func discardNil(ch chan error) chan error {
	r := make(chan error)
	go func() {
		for err := range ch {
			if err == nil {
				continue
			}
			r <- err
		}
		close(r)
	}()
	return r
}
