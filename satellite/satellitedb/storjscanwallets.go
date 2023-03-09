// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments/billing"
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

// GetWallet returns the wallet associated with the given user.
func (walletsDB storjscanWalletsDB) GetWallet(ctx context.Context, userID uuid.UUID) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)
	wallet, err := walletsDB.db.Get_StorjscanWallet_WalletAddress_By_UserId(ctx, dbx.StorjscanWallet_UserId(userID[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return blockchain.Address{}, billing.ErrNoWallet
		}
		return blockchain.Address{}, Error.Wrap(err)
	}
	address, err := blockchain.BytesToAddress(wallet.WalletAddress)
	if err != nil {
		return blockchain.Address{}, Error.Wrap(err)
	}
	return address, nil
}

// GetUser returns the userID associated with the given wallet.
func (walletsDB storjscanWalletsDB) GetUser(ctx context.Context, walletAddress blockchain.Address) (_ uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)
	userID, err := walletsDB.db.Get_StorjscanWallet_UserId_By_WalletAddress(ctx, dbx.StorjscanWallet_WalletAddress(walletAddress.Bytes()))
	if err != nil {
		return uuid.UUID{}, Error.Wrap(err)
	}
	id, err := uuid.FromBytes(userID.UserId)
	if err != nil {
		return uuid.UUID{}, Error.Wrap(err)
	}
	return id, nil
}

// GetAll returns all saved wallet entries.
func (walletsDB storjscanWalletsDB) GetAll(ctx context.Context) (_ []storjscan.Wallet, err error) {
	defer mon.Task()(&ctx)(&err)
	entries, err := walletsDB.db.All_StorjscanWallet(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	var wallets []storjscan.Wallet
	for _, entry := range entries {
		userID, err := uuid.FromBytes(entry.UserId)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		address, err := blockchain.BytesToAddress(entry.WalletAddress)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		wallets = append(wallets, storjscan.Wallet{UserID: userID, Address: address})
	}
	return wallets, nil
}
