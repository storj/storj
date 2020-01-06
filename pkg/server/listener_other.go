// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !linux

package server

import (
	"net"
)

// wrapListener does nothing on this platform.
func wrapListener(lis net.Listener) net.Listener {
	return lis
}
