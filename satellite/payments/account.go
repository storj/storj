// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
)

// Accounts exposes all needed functionality to manage payment accounts.
type Accounts interface {
	// Setup creates a payment account for the user.
	Setup(ctx context.Context, userID uuid.UUID, email string) error
}
