// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"bytes"
	"encoding/gob"
	"net/url"
	"strings"
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

	return url.Parse(s)
}

// CombineErrors combines multiple errors to a single error
func CombineErrors(errs ...error) error { return combinedError(errs) }

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

