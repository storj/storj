// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux && !windows
// +build !linux,!windows

package server

import (
	"go.uber.org/zap"
)

func setTCPFastOpen(fd uintptr, queue int) error { return nil }

func tryInitFastOpen(*zap.Logger) {}
