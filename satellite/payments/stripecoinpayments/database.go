package stripecoinpayments

import (
	"context"
	"github.com/skyrings/skyring-common/tools/uuid"
)

// TODO: return, when lock generator will be able to handle nested interfaces
// type DB interface {
//	StripeCustomers() StripeCustomers
// }

type StripeCustomers interface {
	Insert(ctx context.Context, userID uuid.UUID, customerID string) error
}

