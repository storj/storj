package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// userpaymentinfos allows to work with user payment info storage
type userpaymentinfos struct {
	db dbx.Methods
}

// Create stores user payment info into db
func (infos *userpaymentinfos) Create(ctx context.Context, info console.UserPaymentInfo) (*console.UserPaymentInfo, error) {
	dbxInfo, err := infos.db.Create_UserPaymentInfo(ctx,
		dbx.UserPaymentInfo_UserId(info.UserID[:]),
		dbx.UserPaymentInfo_CustomerId(info.CustomerID))

	if err != nil {
		return nil, err
	}

	return fromDBXUserPaymentInfo(dbxInfo)
}

// Get retrieves one user payment info from storage for particular user
func (infos *userpaymentinfos) Get(ctx context.Context, userID uuid.UUID) (*console.UserPaymentInfo, error) {
	dbxInfo, err := infos.db.Get_UserPaymentInfo_By_UserId(ctx, dbx.UserPaymentInfo_UserId(userID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXUserPaymentInfo(dbxInfo)
}

// fromDBXUserPaymentInfo converts dbx payment info to console.UserPaymentInfo
func fromDBXUserPaymentInfo(info *dbx.UserPaymentInfo) (*console.UserPaymentInfo, error) {
	userID, err := bytesToUUID(info.UserId)
	if err != nil {
		return nil, err
	}

	return &console.UserPaymentInfo{
		UserID:     userID,
		CustomerID: info.CustomerId,
		CreatedAt:  info.CreatedAt,
	}, nil
}
