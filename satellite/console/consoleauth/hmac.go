// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"crypto/hmac"
	"crypto/sha256"
)

// TODO: change to JWT or Macaroon based auth

// Hmac is hmac256 based Signer.
type Hmac struct {
	Secret []byte
}

// Sign implements satellite signer.
func (a *Hmac) Sign(data []byte) ([]byte, error) {
	mac := hmac.New(sha256.New, a.Secret)

	_, err := mac.Write(data)
	if err != nil {
		return nil, err
	}

	return mac.Sum(nil), nil
}
