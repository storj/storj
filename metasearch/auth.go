package metasearch

import (
	"fmt"
	"net/http"
	"strings"

	"storj.io/common/macaroon"
)

// Auth authenticates HTTP requests for metasearch
type Auth interface {
	Authenticate(r *http.Request) error
}

// HeaderAuth authenticates metasearch HTTP requests based on the Authorization header
type HeaderAuth struct {
}

func (a *HeaderAuth) Authenticate(r *http.Request) error {
	// Parse authorization header
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return fmt.Errorf("%w: missing authorization header", ErrAuthorizationFailed)
	}

	// Check for valid authorization
	if !strings.HasPrefix(hdr, "Bearer ") {
		return fmt.Errorf("%w: invalid authorization header", ErrAuthorizationFailed)
	}

	// Parse API token
	rawToken := strings.TrimPrefix(hdr, "Bearer ")
	_, err := macaroon.ParseAPIKey(rawToken)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrAuthorizationFailed, err)
	}

	return nil
}
