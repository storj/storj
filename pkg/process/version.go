// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"time"

	"storj.io/storj/internal/version"
)

const interval = 15 * time.Minute

// LogAndReportVersion logs the current version information
// and reports to monkit
func LogAndReportVersion(ctx context.Context) (err error) {

	err = version.CheckVersion(&ctx)

	ticker := time.NewTicker(interval)

	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err = version.CheckVersion(&ctx)
			return err
		}
	}
}
