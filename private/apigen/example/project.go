// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package example

import (
	"storj.io/common/uuid"
)

// Project contains project information.
type Project struct {
	ID        uuid.UUID `json:"id"`
	OwnerName string    `json:"ownerName"`
}
