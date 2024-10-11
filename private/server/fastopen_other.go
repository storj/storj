// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux && !windows && !freebsd

package server

import (
	"go.uber.org/zap"
)

func setTCPFastOpen(fd uintptr, queue int) error { return nil }

// tryInitFastOpen returns true if fastopen support is possibly enabled.
func tryInitFastOpen(*zap.Logger) bool { return false }
