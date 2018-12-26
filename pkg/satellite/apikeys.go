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
	GetByProjectID(ctx context.Context, projectID uuid.UUID) ([]APIKey, error)
	// Get retrieves APIKey with given ID
	Get(ctx context.Context, id uuid.UUID) (*APIKey, error)
	// Create creates and stores new APIKey
	Create(ctx context.Context, key APIKey) (*APIKey, error)
	// Update updates APIKey in store
	Update(ctx context.Context, key APIKey) error
	// Delete deletes APIKey from store
	Delete(ctx context.Context, id uuid.UUID) error
}

// APIKey describing api key model in the database
type APIKey struct {
	ID uuid.UUID `json:"id"`

	// Fk on project
	ProjectID uuid.UUID `json:"projectId"`

	Key  MockKey `json:"key"`
	Name string  `json:"name"`

	CreatedAt time.Time `json:"createdAt"`
}

// MockKey is a mock api key type
type MockKey [24]byte

// String implements Stringer
func (key MockKey) String() string {
	emptyKey := MockKey{}
	if bytes.Equal(key[:], emptyKey[:]) {
		return ""
	}

	return base64.URLEncoding.EncodeToString(key[:])
}

// MockKeyFromBytes creates new key from byte slice
func MockKeyFromBytes(b []byte) *MockKey {
	key := new(MockKey)
	copy(key[:], b)
	return key
}

// createMockKey creates new mock api key
func createMockKey() (*MockKey, error) {
	key := new(MockKey)

	n, err := io.ReadFull(rand.Reader, key[:])
	if err != nil || n != 24 {
		return nil, errs.New("error creating api key")
	}

	return key, nil
}
