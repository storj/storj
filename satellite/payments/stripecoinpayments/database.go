// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"
	"github.com/skyrings/skyring-common/tools/uuid"
)

type StripeCustomers interface {
	Insert(ctx context.Context, userID uuid.UUID, customerID string) error
}

