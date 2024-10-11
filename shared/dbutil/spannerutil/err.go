// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"cloud.google.com/go/spanner"
	"google.golang.org/grpc/codes"
)

// IsAlreadyExists is true if err code is AlreadyExists.
func IsAlreadyExists(err error) bool {
	return spanner.ErrCode(err) == codes.AlreadyExists
}
