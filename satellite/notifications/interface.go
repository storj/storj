package notifications

import (
	"context"

	"storj.io/storj/pkg/storj"
)

type NotificationDB interface {
	GetAddressByID(ctx context.Context, id storj.NodeID) (address string, err error)
	GetAddressesByIDs(ctx context.Context, ids []storj.NodeID) ([]string, error)
}
