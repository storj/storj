// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !(linux || windows || (darwin && cgo))

package storagenode

import "go.uber.org/zap"

func initializeDiskMon(log *zap.Logger) {}
