// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// SessionPayload is the JSON payload form when an IDP token is embedded.
type SessionPayload struct {
	SessionID       uuid.UUID `json:"sessionID"`
	IDPToken        string    `json:"idpToken"`
	IDPTokenExpiry  time.Time `json:"idpTokenExpiry,omitempty"`
	IDPRefreshToken string    `json:"idpRefreshToken,omitempty"`
}

// ParseSessionPayload parses a token payload into a SessionPayload.
// Old format (16-byte UUID bytes): returns SessionPayload{SessionID: uuid}.
// New format (JSON {"sessionID":"...","idpToken":"...","idpTokenExpiry":"...","idpRefreshToken":"..."}): returns all fields.
func ParseSessionPayload(payload []byte) (SessionPayload, error) {
	sessionID, err := uuid.FromBytes(payload)
	if err == nil {
		return SessionPayload{SessionID: sessionID}, nil
	}
	var p SessionPayload
	if jsonErr := json.Unmarshal(payload, &p); jsonErr != nil {
		return SessionPayload{}, err // return original UUID parse error
	}
	if p.SessionID == (uuid.UUID{}) {
		return SessionPayload{}, errs.New("invalid or missing session ID")
	}
	return p, nil
}

// TODO: change to JWT or Macaroon based auth

// Token represents authentication data structure.
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

// FromBase64URLString creates Token instance from base64URLEncoded string representation.
func FromBase64URLString(token string) (Token, error) {
	i := strings.Index(token, ".")
	if i < 0 {
		return Token{}, errs.New("invalid token format")
	}

	payload := token[:i]
	signature := token[i+1:]

	payloadDecoder := base64.NewDecoder(base64.URLEncoding, bytes.NewReader([]byte(payload)))
	signatureDecoder := base64.NewDecoder(base64.URLEncoding, bytes.NewReader([]byte(signature)))

	payloadBytes, err := io.ReadAll(payloadDecoder)
	if err != nil {
		return Token{}, errs.New("decoding token's signature failed: %s", err)
	}

	signatureBytes, err := io.ReadAll(signatureDecoder)
	if err != nil {
		return Token{}, errs.New("decoding token's body failed: %s", err)
	}

	return Token{Payload: payloadBytes, Signature: signatureBytes}, nil
}
