// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion

import (
	"context"

	"go.uber.org/zap"
)

// Report represents the deleteion status report.
type Report struct {
	Deleted []*ObjectIdentifier
	Failed  []*ObjectIdentifier
}

// HasFailures returns wether a delete operation has failures.
func (r Report) HasFailures() bool {
	return len(r.Failed) > 0
}

// GenerateReport returns the result of a delete, success, or failure.
func GenerateReport(ctx context.Context, log *zap.Logger, requests []*ObjectIdentifier, deletedPaths [][]byte) Report {
	defer mon.Task()(&ctx)(nil)

	report := Report{}
	deletedObjects := make(map[string]*ObjectIdentifier)
	for _, path := range deletedPaths {
		if path == nil {
			continue
		}
		id, _, err := ParseSegmentPath(path)
		if err != nil {
			log.Debug("failed to parse deleted segmnt path for report",
				zap.String("Raw Segment Path", string(path)),
			)
			continue
		}
		if _, ok := deletedObjects[id.Key()]; !ok {
			deletedObjects[id.Key()] = &id
		}
	}

	// populate report with failed and deleted objects
	for _, req := range requests {
		if _, ok := deletedObjects[req.Key()]; !ok {
			report.Failed = append(report.Failed, req)
		} else {
			report.Deleted = append(report.Deleted, req)
		}
	}
	return report
}
