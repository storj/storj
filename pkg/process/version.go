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
func LogAndReportVersion(ctx context.Context) error {
	return nil
}

func checkVersion(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
		_, err = version.QueryVersionFromControlServer()
		return
	}
}
