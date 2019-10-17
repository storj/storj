// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"net/url"

	"github.com/zeebo/errs"
)

// GetTransacationKeyFromURL parses provided raw url string
// and extracts authorization key from it.
func GetTransacationKeyFromURL(rawurl string) (string, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return "", errs.Wrap(err)
	}

	key := u.Query().Get("key")
	if key == "" {
		return "", errs.New("no key value found")
	}

	return key, nil
}
