// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/skyrings/skyring-common/tools/uuid"
)

// ProjectCharge shows how much money current project will charge in the end of the month.
type ProjectCharge struct {
	ProjectID uuid.UUID `json:"projectId"`
	// StorageGbHrs shows how much cents we should pay for storing GB*Hrs.
	StorageGbHrs int64 `json:"storage"`
	// Egress shows how many cents we should pay for Egress.
	Egress int64 `json:"egress"`
	// ObjectCount shows how many cents we should pay for objects count.
	ObjectCount int64 `json:"objectCount"`
}
