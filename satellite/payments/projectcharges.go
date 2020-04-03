// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
)

// ProjectCharge shows project usage and how much money current project will charge in the end of the month.
type ProjectCharge struct {
	accounting.ProjectUsage

	ProjectID uuid.UUID `json:"projectId"`
	// StorageGbHrs shows how much cents we should pay for storing GB*Hrs.
	StorageGbHrs int64 `json:"storagePrice"`
	// Egress shows how many cents we should pay for Egress.
	Egress int64 `json:"egressPrice"`
	// ObjectCount shows how many cents we should pay for objects count.
	ObjectCount int64 `json:"objectPrice"`
}
