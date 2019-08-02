// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

func init() {
	monkit.AddErrorNameHandler(func(err error) (string, bool) {
		if s, ok := status.FromError(err); ok {
			return "grpc_" + s.Code().String(), true
		}
		return "", false
	})
}
