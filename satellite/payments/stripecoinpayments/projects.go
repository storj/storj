// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
)

type Projects interface {
	List(ctx context.Context, userID uuid.UUID, createdBefore time.Time) ([]console.Project, error)
}
