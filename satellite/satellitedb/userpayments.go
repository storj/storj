// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments/stripepayments"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

// userpayments is the an implementation of stripepayments.UserPayments.
// Allows to work with user payment info storage
type userpayments struct {
	db dbx.Methods
}

// Create stores user payment info into db
func (infos *userpayments) Create(ctx context.Context, info stripepayments.UserPayment) (*stripepayments.UserPayment, error) {
	dbxInfo, err := infos.db.Create_UserPayment(ctx,
		dbx.UserPayment_UserId(info.UserID[:]),
		dbx.UserPayment_CustomerId(info.CustomerID))

	if err != nil {
		return nil, err
	}

	return fromDBXUserPayment(dbxInfo)
}

// Get retrieves one user payment info from storage for particular user
func (infos *userpayments) Get(ctx context.Context, userID uuid.UUID) (*stripepayments.UserPayment, error) {
	dbxInfo, err := infos.db.Get_UserPayment_By_UserId(ctx, dbx.UserPayment_UserId(userID[:]))
	if err != nil {
		return nil, err
	}

	return fromDBXUserPayment(dbxInfo)
}

// fromDBXUserPayment converts dbx user payment info to *stripepayments.UserPayment
func fromDBXUserPayment(info *dbx.UserPayment) (*stripepayments.UserPayment, error) {
	userID, err := bytesToUUID(info.UserId)
	if err != nil {
		return nil, err
	}

	return &stripepayments.UserPayment{
		UserID:     userID,
		CustomerID: info.CustomerId,
		CreatedAt:  info.CreatedAt,
	}, nil
}
