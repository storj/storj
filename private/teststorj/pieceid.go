// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package teststorj

import (
	"storj.io/common/storj"
)

// PieceIDFromBytes converts a byte slice into a piece ID
func PieceIDFromBytes(b []byte) storj.PieceID {
	id, _ := storj.PieceIDFromBytes(fit(b))
	return id
}

// PieceIDFromString decodes a hex encoded piece ID string
func PieceIDFromString(s string) storj.PieceID {
	return PieceIDFromBytes([]byte(s))
}
