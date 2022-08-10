// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments"
)

// ErrService is storjscan service error class.
var ErrService = errs.Class("storjscan service")

// ensures that Wallets implements payments.Wallets.
var _ payments.DepositWallets = (*Service)(nil)

// Service exposes API to interact with storjscan payments provider.
type Service struct {
	log        *zap.Logger
	walletsDB  WalletsDB
	paymentsDB PaymentsDB
	client     *Client
}

// NewService creates new storjscan service instance.
func NewService(log *zap.Logger, walletsDB WalletsDB, paymentsDB PaymentsDB, client *Client) *Service {
	return &Service{
		log:        log,
		walletsDB:  walletsDB,
		paymentsDB: paymentsDB,
		client:     client,
	}
}

// Claim gets a new crypto wallet and associates it with a user.
func (service *Service) Claim(ctx context.Context, userID uuid.UUID) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)

	address, err := service.client.ClaimNewEthAddress(ctx)
	if err != nil {
		return blockchain.Address{}, ErrService.Wrap(err)
	}
	err = service.walletsDB.Add(ctx, userID, address)
	if err != nil {
		return blockchain.Address{}, ErrService.Wrap(err)
	}

	return address, nil
}

// Get returns the crypto wallet address associated with the given user.
func (service *Service) Get(ctx context.Context, userID uuid.UUID) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)

	address, err := service.walletsDB.Get(ctx, userID)
	return address, ErrService.Wrap(err)
}

// Payments retrieves payments for specific wallet.
func (service *Service) Payments(ctx context.Context, wallet blockchain.Address, limit int, offset int64) (_ []payments.WalletPayment, err error) {
	defer mon.Task()(&ctx)(&err)

	cachedPayments, err := service.paymentsDB.ListWallet(ctx, wallet, limit, offset)
	if err != nil {
		return nil, ErrService.Wrap(err)
	}

	var walletPayments []payments.WalletPayment
	for _, pmnt := range cachedPayments {
		walletPayments = append(walletPayments, payments.WalletPayment{
			From:        pmnt.From,
			To:          pmnt.To,
			TokenValue:  pmnt.TokenValue,
			USDValue:    pmnt.USDValue,
			Status:      pmnt.Status,
			BlockHash:   pmnt.BlockHash,
			BlockNumber: pmnt.BlockNumber,
			Transaction: pmnt.Transaction,
			LogIndex:    pmnt.LogIndex,
			Timestamp:   pmnt.Timestamp,
		})
	}

	return walletPayments, nil
}
