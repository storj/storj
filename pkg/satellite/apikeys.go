// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"io"
	"time"

	"github.com/zeebo/errs"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// APIKeys is interface for working with api keys store
type APIKeys interface {
	// GetByProjectID retrieves list of APIKeys for given projectID
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]APIKeyInfo, error)
	// Get retrieves APIKeyInfo with given ID
	Get(ctx context.Context, id uuid.UUID) (*APIKeyInfo, error)
	// Create creates and stores new APIKeyInfo
	Create(ctx context.Context, key APIKey, info APIKeyInfo) (*APIKeyInfo, error)
	// Update updates APIKeyInfo in store
	Update(ctx context.Context, key APIKeyInfo) error
	// Delete deletes APIKeyInfo from store
	Delete(ctx context.Context, id uuid.UUID) error
}

// APIKeyInfo describing api key model in the database
type APIKeyInfo struct {
	ID uuid.UUID `json:"id"`

	// Fk on project
	ProjectID uuid.UUID `json:"projectId"`

	Name string `json:"name"`

	CreatedAt time.Time `json:"createdAt"`
}

// APIKey is a mock api key type
type APIKey [24]byte

// String implements Stringer
func (key APIKey) String() string {
	emptyKey := APIKey{}
	if bytes.Equal(key[:], emptyKey[:]) {
		return ""
	}

	return base64.URLEncoding.EncodeToString(key[:])
}

// APIKeyFromBytes creates new key from byte slice
func APIKeyFromBytes(b []byte) *APIKey {
	key := new(APIKey)
	copy(key[:], b)
	return key
}

// createAPIKey creates new mock api key
func createAPIKey() (*APIKey, error) {
	key := new(APIKey)

	n, err := io.ReadFull(rand.Reader, key[:])
	if err != nil || n != 24 {
		return nil, errs.New("error creating api key")
	}

	return key, nil
}
