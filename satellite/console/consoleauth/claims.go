// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleauth

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
)

//TODO: change to JWT or Macaroon based auth

// Claims represents data signed by server and used for authentication
type Claims struct {
	ID         uuid.UUID `json:"id"`
	Email      string    `json:"email,omitempty"`
	Expiration time.Time `json:"expires,omitempty"`
}

// JSON returns json representation of Claims
func (c *Claims) JSON() ([]byte, error) {
	buffer := bytes.NewBuffer(nil)

	err := json.NewEncoder(buffer).Encode(c)
	return buffer.Bytes(), err
}

// FromJSON returns Claims instance, parsed from JSON
func FromJSON(data []byte) (*Claims, error) {
	claims := new(Claims)

	err := json.NewDecoder(bytes.NewReader(data)).Decode(claims)
	if err != nil {
		return nil, err
	}

	return claims, nil
}
