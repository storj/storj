// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import "storj.io/common/macaroon"

// ActionType specifies the operation type being performed that the Macaroon will validate
type ActionType = macaroon.ActionType

const (
	// ActionRead specifies a read operation
	ActionRead = macaroon.ActionRead
	// ActionWrite specifies a write operation
	ActionWrite = macaroon.ActionWrite
	// ActionList specifies a list operation
	ActionList = macaroon.ActionList
	// ActionDelete specifies a delete operation
	ActionDelete = macaroon.ActionDelete
	// ActionProjectInfo requests project-level information
	ActionProjectInfo = macaroon.ActionProjectInfo
)

// Action specifies the specific operation being performed that the Macaroon will validate
type Action = macaroon.Action

// APIKey implements a Macaroon-backed Storj-v3 API key.
type APIKey = macaroon.APIKey

// ParseAPIKey parses a given api key string and returns an APIKey if the
// APIKey was correctly formatted. It does not validate the key.
func ParseAPIKey(key string) (*APIKey, error) {
	return macaroon.ParseAPIKey(key)
}

// ParseRawAPIKey parses raw api key data and returns an APIKey if the APIKey
// was correctly formatted. It does not validate the key.
func ParseRawAPIKey(data []byte) (*APIKey, error) {
	return macaroon.ParseRawAPIKey(data)
}

// NewAPIKey generates a brand new unrestricted API key given the provided
// server project secret
func NewAPIKey(secret []byte) (*APIKey, error) {
	return macaroon.NewAPIKey(secret)
}

// AllowedBuckets stores information about which buckets are
// allowed to be accessed, where `Buckets` stores names of buckets that are
// allowed and `All` is a bool that indicates if all buckets are allowed or not
type AllowedBuckets = macaroon.AllowedBuckets

// NewCaveat returns a Caveat with a random generated nonce.
func NewCaveat() (Caveat, error) {
	return macaroon.NewCaveat()
}

// Macaroon is a struct that determine contextual caveats and authorization
type Macaroon = macaroon.Macaroon

// NewUnrestricted creates Macaroon with random Head and generated Tail
func NewUnrestricted(secret []byte) (*Macaroon, error) {
	return macaroon.NewUnrestricted(secret)
}

// NewSecret generates cryptographically random 32 bytes
func NewSecret() (secret []byte, err error) {
	return macaroon.NewSecret()
}

// ParseMacaroon converts binary to macaroon
func ParseMacaroon(data []byte) (_ *Macaroon, err error) {
	return macaroon.ParseMacaroon(data)
}

// Caveat is a caveat.
type Caveat = macaroon.Caveat

// Caveat_Path is a path for caveat.
// If any entries exist, require all access to happen in at least
// one of them.
type Caveat_Path = macaroon.Caveat_Path //nolint alias to generated code
