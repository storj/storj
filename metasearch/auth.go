package metasearch

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

// Auth authenticates HTTP requests for metasearch
type Auth interface {
	Authenticate(ctx context.Context, r *http.Request) (projectID uuid.UUID, err error)
}

// HeaderAuth authenticates metasearch HTTP requests based on the Authorization header
type HeaderAuth struct {
	db satellite.DB
}

func NewHeaderAuth(db satellite.DB) *HeaderAuth {
	return &HeaderAuth{
		db: db,
	}
}

func (a *HeaderAuth) Authenticate(ctx context.Context, r *http.Request) (projectID uuid.UUID, err error) {
	// Parse authorization header
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		err = fmt.Errorf("%w: missing authorization header", ErrAuthorizationFailed)
		return
	}

	// Check for valid authorization
	if !strings.HasPrefix(hdr, "Bearer ") {
		err = fmt.Errorf("%w: invalid authorization header", ErrAuthorizationFailed)
		return
	}

	// Parse API token
	rawToken := strings.TrimPrefix(hdr, "Bearer ")
	apiKey, err := macaroon.ParseAPIKey(rawToken)
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrAuthorizationFailed, err)
		return
	}

	// Get projectId
	var keyInfo *console.APIKeyInfo
	keyInfo, err = a.db.Console().APIKeys().GetByHead(ctx, apiKey.Head())
	if err != nil {
		err = fmt.Errorf("%w: %s", ErrAuthorizationFailed, err)
		return
	}
	projectID = keyInfo.ProjectID
	return
}
