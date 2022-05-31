// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"

	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments/storjscan"
	"storj.io/storj/satellite/satellitedb/dbx"
)

// ensure that storjscanWalletsDB implements storjscan.WalletsDB.
var _ storjscan.WalletsDB = (*storjscanWalletsDB)(nil)

// storjscanWalletsDB is Storjscan wallets DB.
//
// architecture: Database
type storjscanWalletsDB struct {
	db *satelliteDB
}

// Add creates new user/wallet association record.
func (walletsDB storjscanWalletsDB) Add(ctx context.Context, userID uuid.UUID, walletAddress blockchain.Address) (err error) {
	defer mon.Task()(&ctx)(&err)
	return walletsDB.db.CreateNoReturn_StorjscanWallet(ctx,
		dbx.StorjscanWallet_UserId(userID[:]),
		dbx.StorjscanWallet_WalletAddress(walletAddress.Bytes()))
}

// Get returns with thw wallet associated with the user.
func (walletsDB storjscanWalletsDB) Get(ctx context.Context, userID uuid.UUID) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)
	wallet, err := walletsDB.db.Get_StorjscanWallet_WalletAddress_By_UserId(ctx, dbx.StorjscanWallet_UserId(userID[:]))
	if err != nil {
		return blockchain.Address{}, Error.Wrap(err)
	}
	address, err := blockchain.BytesToAddress(wallet.WalletAddress)
	if err != nil {
		return blockchain.Address{}, Error.Wrap(err)
	}
	return address, nil
}

// GetAllUsers returns with all the users which has associated wallet.
func (walletsDB storjscanWalletsDB) GetAllUsers(ctx context.Context) (_ []uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)
	users, err := walletsDB.db.All_StorjscanWallet_UserId(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	var userIDs []uuid.UUID
	for _, user := range users {
		userID, err := uuid.FromBytes(user.UserId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		userIDs = append(userIDs, userID)
	}
	return userIDs, nil
}
