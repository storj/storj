// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"strings"

	"github.com/zeebo/errs"
)

//TODO: change to JWT or Macaroon based auth

// Token represents authentication data structure
type Token struct {
	Payload   []byte
	Signature []byte
}

// String returns base64URLEncoded data joined with .
func (t Token) String() string {
	payload := base64.URLEncoding.EncodeToString(t.Payload)
	signature := base64.URLEncoding.EncodeToString(t.Signature)

	return strings.Join([]string{payload, signature}, ".")
}

// FromBase64URLString creates Token instance from base64URLEncoded string representation
func FromBase64URLString(token string) (Token, error) {
	i := strings.Index(token, ".")
	if i < 0 {
		return Token{}, errs.New("invalid token format")
	}

	payload := token[:i]
	signature := token[i+1:]

	payloadDecoder := base64.NewDecoder(base64.URLEncoding, bytes.NewReader([]byte(payload)))
	signatureDecoder := base64.NewDecoder(base64.URLEncoding, bytes.NewReader([]byte(signature)))

	payloadBytes, err := ioutil.ReadAll(payloadDecoder)
	if err != nil {
		return Token{}, errs.New("decoding token's signature failed: %s", err)
	}

	signatureBytes, err := ioutil.ReadAll(signatureDecoder)
	if err != nil {
		return Token{}, errs.New("decoding token's body failed: %s", err)
	}

	return Token{Payload: payloadBytes, Signature: signatureBytes}, nil
}
