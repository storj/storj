// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/skyrings/skyring-common/tools/uuid"
)

// Accounts -
type Accounts interface {
	Setup(userID uuid.UUID) error
}

// Account -
type Account struct {
	ID     string
	UserID uuid.UUID
}

// AccountInfo - stores all needed information needed for account creation
type AccountInfo struct {
	FullName    string
	Email       string
	Description string
}
