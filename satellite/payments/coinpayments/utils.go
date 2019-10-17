// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"net/url"

	"github.com/zeebo/errs"
)

// ErrNoAuthorizationKey is error that indicates that there is no authorization key.
var ErrNoAuthorizationKey = Error.New("no authorization key")

// GetTransacationKeyFromURL parses provided raw url string
// and extracts authorization key from it. Returns ErrNoAuthorizationKey if
// there is no authorization key and error if rawurl cannot be parsed.
func GetTransacationKeyFromURL(rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", errs.Wrap(err)
	}

	key := u.Query().Get("key")
	if key == "" {
		return "", ErrNoAuthorizationKey
	}

	return key, nil
}
