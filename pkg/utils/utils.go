// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"github.com/zeebo/errs"
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

// SplitDBURL returns the driver and DSN portions of a URL
func SplitDBURL(s string) (string, string, error) {
	// consider https://github.com/xo/dburl if this ends up lacking
	parts := strings.SplitN(s, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Could not parse DB URL %s", s)
	}
	if parts[0] == "postgres" {
		parts[1] = s // postgres wants full URLS for its DSN
	}
	return parts[0], parts[1], nil
}

// CombineErrors combines multiple errors to a single error
var CombineErrors = errs.Combine

// ErrorGroup contains a set of non-nil errors
type ErrorGroup struct {
	e errs.Group
}

// Add add errs to ErrorGroup
func (e *ErrorGroup) Add(err ...error) {
	e.e.Add(err...)
}

// Finish compiles the given errors into a single error, or nil if there were
// none
func (e *ErrorGroup) Finish() error {
	return e.e.Err()
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

// RunJointly runs the given jobs concurrently. As soon as one finishes,
// the timeout begins ticking. If all jobs finish before the deadline
// RunJointly returns the combined errors. If some jobs time out, a timeout
// error is returned for them.
func RunJointly(timeout time.Duration, jobs ...func() error) error {
	ch := make(chan error, len(jobs))

	for _, job := range jobs {
		go func(job func() error) {
			// run the job but turn panics into errors
			err := func() (err error) {
				defer func() {
					if rec := recover(); rec != nil {
						err = errs.New("panic: %+v", rec)
					}
				}()
				return job()
			}()
			// send the error
			ch <- err
		}(job)
	}

	errgroup := make([]error, 0, len(jobs))
	errgroup = append(errgroup, <-ch)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

loop:
	for len(errgroup) < len(jobs) {
		select {
		case err := <-ch:
			errgroup = append(errgroup, err)
		case <-timer.C:
			errgroup = append(errgroup,
				errs.New("%d jobs timed out", len(jobs)-len(errgroup)))
			break loop
		}
	}

	return errs.Combine(errgroup...)
}
