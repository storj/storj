// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"bytes"
	"context"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
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

	mon = monkit.Package()
)

// ActionType specifies the operation type being performed that the Macaroon will validate
type ActionType int

const (
	// not using iota because these values are persisted in macaroons
	_ ActionType = 0

	// ActionRead specifies a read operation
	ActionRead ActionType = 1
	// ActionWrite specifies a read operation
	ActionWrite ActionType = 2
	// ActionList specifies a read operation
	ActionList ActionType = 3
	// ActionDelete specifies a read operation
	ActionDelete ActionType = 4
	// ActionProjectInfo requests project-level information
	ActionProjectInfo ActionType = 5
)

// Action specifies the specific operation being performed that the Macaroon will validate
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

// ParseRawAPIKey parses raw api key data and returns an APIKey if the APIKey
// was correctly formatted. It does not validate the key.
func ParseRawAPIKey(data []byte) (*APIKey, error) {
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
func (a *APIKey) Check(ctx context.Context, secret []byte, action Action, revoked [][]byte) (err error) {
	defer mon.Task()(&ctx)(&err)
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

// AllowedBuckets stores information about which buckets are
// allowed to be accessed, where `Buckets` stores names of buckets that are
// allowed and `All` is a bool that indicates if all buckets are allowed or not
type AllowedBuckets struct {
	All     bool
	Buckets map[string]struct{}
}

// GetAllowedBuckets returns a list of all the allowed bucket paths that match the Action operation
func (a *APIKey) GetAllowedBuckets(ctx context.Context, action Action) (allowed AllowedBuckets, err error) {
	defer mon.Task()(&ctx)(&err)

	caveats := a.mac.Caveats()
	// if there aren't any caveats, then all buckets are allowed
	if len(caveats) == 0 {
		allowed.All = true
		return allowed, err
	}

	allowed.Buckets = map[string]struct{}{}
	for _, cavbuf := range caveats {
		var cav Caveat
		err := proto.Unmarshal(cavbuf, &cav)
		if err != nil {
			return allowed, ErrFormat.New("invalid caveat format: %v", err)
		}
		if cav.Allows(action) {
			for _, caveatPath := range cav.AllowedPaths {
				allowed.Buckets[string(caveatPath.Bucket)] = struct{}{}
			}
		}
	}

	return allowed, err
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
func (a *APIKey) Serialize() string {
	return base58.CheckEncode(a.mac.Serialize(), 0)
}

// SerializeRaw serialize the API Key to raw bytes
func (a *APIKey) SerializeRaw() []byte {
	return a.mac.Serialize()
}

// Allows returns true if the provided action is allowed by the caveat.
func (c *Caveat) Allows(action Action) bool {
	// if the action is after the caveat's "not after" field, then it is invalid
	if c.NotAfter != nil && action.Time.After(*c.NotAfter) {
		return false
	}
	// if the caveat's "not before" field is *after* the action, then the action
	// is before the "not before" field and it is invalid
	if c.NotBefore != nil && c.NotBefore.After(action.Time) {
		return false
	}

	// we want to always allow reads for bucket metadata, perhaps filtered by the
	// buckets in the allowed paths.
	if action.Op == ActionRead && len(action.EncryptedPath) == 0 {
		if len(c.AllowedPaths) == 0 {
			return true
		}
		if len(action.Bucket) == 0 {
			// if no action.bucket name is provided, then this call is checking that
			// we can list all buckets. In that case, return true here and we will
			// filter out buckets that aren't allowed later with `GetAllowedBuckets()`
			return true
		}
		for _, path := range c.AllowedPaths {
			if bytes.Equal(path.Bucket, action.Bucket) {
				return true
			}
		}
		return false
	}

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
	case ActionProjectInfo:
		// allow
	default:
		return false
	}

	if len(c.AllowedPaths) > 0 && action.Op != ActionProjectInfo {
		found := false
		for _, path := range c.AllowedPaths {
			if bytes.Equal(action.Bucket, path.Bucket) &&
				bytes.HasPrefix(action.EncryptedPath, path.EncryptedPathPrefix) {
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
