package satellitedb

import (
	"context"
	"database/sql"

	"storj.io/storj/satellite/notifications"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

var _ notifications.NotificationDB = (*NotificationDB)(nil)

type NotificationDB struct {
	db *dbx.DB
}

func (notification *NotificationDB) GetAddressByID(ctx context.Context, id storj.NodeID) (address string, err error) {
	defer mon.Task()(&ctx)(&err)

	if id.IsZero() {
		return "", nil
	}

	node, err := notification.db.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if err == sql.ErrNoRows {
		return "", errs.New("No rows found")
	}
	if err != nil {
		return "", err
	}

	return node.Address, nil
}

func (notification *NotificationDB) GetAddressesByIDs(ctx context.Context, ids []storj.NodeID) (addresses []string, err error) {
	for i := range ids {
		address, err := notification.GetAddressByID(ctx, ids[i])
		if err != nil {
			return nil, err
		}

		addresses = append(addresses, address)
	}

	return addresses, nil
}
