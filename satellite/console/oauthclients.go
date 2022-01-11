// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"

	"storj.io/common/uuid"
)

// OAuthClients defines an interface for creating, updating, and obtaining information about oauth clients known to our
// system.
type OAuthClients interface {
	// Get returns the OAuthClient associated with the provided id.
	Get(ctx context.Context, id uuid.UUID) (OAuthClient, error)

	// Create creates a new OAuthClient.
	Create(ctx context.Context, client OAuthClient) error

	// Update modifies information for the provided OAuthClient.
	Update(ctx context.Context, client OAuthClient) error

	// Delete deletes the identified client from the database.
	Delete(ctx context.Context, id uuid.UUID) error
}

// OAuthClient defines a concrete representation of an oauth client.
type OAuthClient struct {
	ID          uuid.UUID `json:"id"`
	Secret      []byte    `json:"secret"`
	UserID      uuid.UUID `json:"userID"`
	RedirectURL string    `json:"redirectURL"`
	AppName     string    `json:"appName"`
	AppLogoURL  string    `json:"appLogoURL"`
}
