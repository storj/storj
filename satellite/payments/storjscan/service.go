// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

var (
	// Error defines storjscan service error.
	Error = errs.Class("storjscan service")
)

// ensures that Wallets implements payments.Wallets.
var _ payments.DepositWallets = (*Service)(nil)

// Config stores needed information for storjscan service initialization.
type Config struct {
	Credentials coinpayments.Credentials
	Endpoint    string
}

// Service is an implementation for payment service via Stripe and Coinpayments.
//
// architecture: Service
type Service struct {
	walletsDB       WalletsDB
	storjscanClient *Client
}

// NewService creates a Service instance.
func NewService(db DB, storjscanClient *Client) (*Service, error) {
	return &Service{
		walletsDB:       db.Wallets(),
		storjscanClient: storjscanClient,
	}, nil
}

// Claim gets a new crypto wallet and associates it with a user.
func (service *Service) Claim(ctx context.Context, userID uuid.UUID) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)

	address, err := service.storjscanClient.ClaimNewEthAddress(ctx)
	if err != nil {
		return blockchain.Address{}, err
	}
	err = service.walletsDB.Add(ctx, userID, address)
	return address, err
}

// Get returns the crypto wallet address associated with the given user.
func (service *Service) Get(ctx context.Context, userID uuid.UUID) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)

	return service.walletsDB.Get(ctx, userID)
}
