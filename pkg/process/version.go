// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"time"

	"go.uber.org/zap"
	"storj.io/storj/internal/version"
)

const interval = 15 * time.Minute

// LogAndReportVersion logs the current version information
// and reports to monkit
func LogAndReportVersion(ctx context.Context) {
	if err := version.CheckVersion(&ctx); err != nil {
		zap.S().Error("Failed to check version: ", err)
	}

	ticker := time.NewTicker(interval)

	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			err := version.CheckVersion(&ctx)
			zap.S().Error("Failed to check version: ", err)
		}
	}
}
