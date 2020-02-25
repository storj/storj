// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package debug

import (
	"github.com/spacemonkeygo/monkit/v3"
	"google.golang.org/grpc/status"
)

func init() {
	monkit.AddErrorNameHandler(func(err error) (string, bool) {
		if s, ok := status.FromError(err); ok {
			return "grpc_" + s.Code().String(), true
		}
		return "", false
	})
}
