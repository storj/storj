// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package consoledb

import (
	"storj.io/common/uuid"
)

// uuidsToBytesArray converts []uuid.UUID into [][]byte.
func uuidsToBytesArray(uuidArr []uuid.UUID) (bytesArr [][]byte) {
	for _, v := range uuidArr {
		bytesArr = append(bytesArr, v.Bytes())
	}
	return
}
