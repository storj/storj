// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"bytes"
	time "time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
)

var (
	// Error is a general API Key error
	Error = errs.Class("api key error")
	// ErrFormat means that the structural formatting of the API Key is invalid
	ErrFormat = errs.Class("api key format error")
	// ErrInvalid means that the API Key is improperly signed
	ErrInvalid = errs.Class("api key invalid error")
	// ErrUnauthorized means that the API key does not grant the requested permission
	ErrUnauthorized = errs.Class("api key unauthorized error")
	// ErrRevoked means the API key has been revoked
	ErrRevoked = errs.Class("api key revocation error")
)

// ActionType specifies the operation type being performed that the Macaroon will validate
type ActionType int

const (
	_ ActionType = iota // ActionType zero value

	// ActionRead specifies a read operation
	ActionRead
	// ActionWrite specifies a read operation
	ActionWrite
	// ActionList specifies a read operation
	ActionList
	// ActionDelete specifies a read operation
	ActionDelete
)

// ActionType specifies the specific operation being performed that the Macaroon will validate
type Action struct {
	Op            ActionType
	Bucket        []byte
	EncryptedPath []byte
	Time          time.Time
}

// APIKey implements a Macaroon-backed Storj-v3 API key.
type APIKey struct {
	mac *Macaroon
}

// ParseAPIKey parses a given api key string and returns an APIKey if the
// APIKey was correctly formatted. It does not validate the key.
func ParseAPIKey(key string) (*APIKey, error) {
	data, version, err := base58.CheckDecode(key)
	if err != nil || version != 0 {
		return nil, ErrFormat.New("invalid api key format")
	}
	mac, err := ParseMacaroon(data)
	if err != nil {
		return nil, ErrFormat.Wrap(err)
	}
	return &APIKey{mac: mac}, nil
}

// NewAPIKey generates a brand new unrestricted API key given the provided
// server project secret
func NewAPIKey(secret []byte) (*APIKey, error) {
	mac, err := NewUnrestricted(secret)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &APIKey{mac: mac}, nil
}

// Check makes sure that the key authorizes the provided action given the root
// project secret and any possible revocations, returning an error if the action
// is not authorized. 'revoked' is a list of revoked heads.
func (a *APIKey) Check(secret []byte, action Action, revoked [][]byte) error {
	if !a.mac.Validate(secret) {
		return ErrInvalid.New("macaroon unauthorized")
	}

	// a timestamp is always required on an action
	if action.Time.IsZero() {
		return Error.New("no timestamp provided")
	}

	caveats := a.mac.Caveats()
	for _, cavbuf := range caveats {
		var cav Caveat
		err := proto.Unmarshal(cavbuf, &cav)
		if err != nil {
			return ErrFormat.New("invalid caveat format")
		}
		if !cav.Allows(action) {
			return ErrUnauthorized.New("action disallowed")
		}
	}

	head := a.mac.Head()
	for _, revokedID := range revoked {
		if bytes.Equal(revokedID, head) {
			return ErrRevoked.New("macaroon head revoked")
		}
	}

	return nil
}

// Restrict generates a new APIKey with the provided Caveat attached.
func (a *APIKey) Restrict(caveat Caveat) (*APIKey, error) {
	buf, err := proto.Marshal(&caveat)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	mac, err := a.mac.AddFirstPartyCaveat(buf)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return &APIKey{mac: mac}, nil
}

// Head returns the identifier for this macaroon's root ancestor.
func (a *APIKey) Head() []byte {
	return a.mac.Head()
}

// Tail returns the identifier for this macaroon only.
func (a *APIKey) Tail() []byte {
	return a.mac.Tail()
}

// Serialize serializes the API Key to a string
func (a *APIKey) Serialize() (string, error) {
	return base58.CheckEncode(a.mac.Serialize(), 0), nil
}

// Allows returns true if the provided action is allowed by the caveat.
func (c *Caveat) Allows(action Action) bool {
	switch action.Op {
	case ActionRead:
		if c.DisallowReads {
			return false
		}
	case ActionWrite:
		if c.DisallowWrites {
			return false
		}
	case ActionList:
		if c.DisallowLists {
			return false
		}
	case ActionDelete:
		if c.DisallowDeletes {
			return false
		}
	default:
		return false
	}

	// if the action is after the caveat's "not after" field, then it is invalid
	if c.NotAfter != nil && action.Time.After(*c.NotAfter) {
		return false
	}
	// if the caveat's "not before" field is *after* the action, then the action
	// is before the "not before" field and it is invalid
	if c.NotBefore != nil && c.NotBefore.After(action.Time) {
		return false
	}

	if len(c.Buckets) > 0 {
		found := false
		for _, bucket := range c.Buckets {
			if bytes.Equal(action.Bucket, bucket) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(c.EncryptedPathPrefixes) > 0 {
		found := false
		for _, path := range c.EncryptedPathPrefixes {
			if bytes.HasPrefix(action.EncryptedPath, path) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}
