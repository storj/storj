// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"time"

	"storj.io/storj/internal/version"
)

// LogAndReportVersion logs the current version information
// and reports to monkit
func LogAndReportVersion(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = version.CheckVersion()

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	select {
	case <-ctx.Done():
		// ToDO: Handle
	case <-ticker.C:
		err = version.CheckVersion()
	}
	return
}
