// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

import (
	"github.com/skyrings/skyring-common/tools/uuid"
)

// bucket_bandwidth_rollup bucket_storage_tally

// ProjectCharge shows how much money current project will charge in the end of the month.
type ProjectCharge struct {
	ProjectID uuid.UUID `json:"projectId"`
	Amount    int64     `json:"amount"`
}
