// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
)

// BytesToUUID is used to convert []byte to UUID.
func BytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}
