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
