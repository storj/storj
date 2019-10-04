// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/skyrings/skyring-common/tools/uuid"
)

// Accounts exposes all needed functionality to manage payment accounts
type Accounts interface {
	Setup(userID uuid.UUID) error
}

// Account stores all payment related information
type Account struct {
	Balance float64
}
