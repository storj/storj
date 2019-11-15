// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripecoinpayments

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

// ensure that storjTokens implements payments.StorjTokens.
var _ payments.StorjTokens = (*storjTokens)(nil)

// storjTokens implements payments.StorjTokens.
type storjTokens struct {
	service *Service
}

// Deposit creates new deposit transaction with the given amount returning
// ETH wallet address where funds should be sent. There is one
// hour limit to complete the transaction. Transaction is saved to DB with
// reference to the user who made the deposit.
func (tokens *storjTokens) Deposit(ctx context.Context, userID uuid.UUID, amount *payments.TokenAmount) (_ *payments.Transaction, err error) {
	defer mon.Task()(&ctx, userID, amount)(&err)

	customerID, err := tokens.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	c, err := tokens.service.stripeClient.Customers.Get(customerID, nil)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	rate, err := tokens.service.GetRate(ctx, coinpayments.CurrencyLTCT, coinpayments.CurrencyUSD)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	tx, err := tokens.service.coinPayments.Transactions().Create(ctx,
		&coinpayments.CreateTX{
			Amount:      *amount.BigFloat(),
			CurrencyIn:  coinpayments.CurrencyLTCT,
			CurrencyOut: coinpayments.CurrencyLTCT,
			BuyerEmail:  c.Email,
		},
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	key, err := coinpayments.GetTransacationKeyFromURL(tx.CheckoutURL)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if err = tokens.service.db.Transactions().LockRate(ctx, tx.ID, rate); err != nil {
		return nil, Error.Wrap(err)
	}

	cpTX, err := tokens.service.db.Transactions().Insert(ctx,
		Transaction{
			ID:        tx.ID,
			AccountID: userID,
			Address:   tx.Address,
			Amount:    tx.Amount,
			Status:    coinpayments.StatusPending,
			Key:       key,
			Timeout:   tx.Timeout,
		},
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &payments.Transaction{
		ID:        payments.TransactionID(tx.ID),
		AccountID: userID,
		Amount:    *payments.TokenAmountFromBigFloat(&tx.Amount),
		Received:  *payments.NewTokenAmount(),
		Address:   tx.Address,
		Status:    payments.TransactionStatusPending,
		Timeout:   tx.Timeout,
		CreatedAt: cpTX.CreatedAt,
	}, nil
}

// ListTransactionInfos fetches all transactions from the database for specified user, reconstructing checkout link.
func (tokens *storjTokens) ListTransactionInfos(ctx context.Context, userID uuid.UUID) (_ []payments.TransactionInfo, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	txs, err := tokens.service.db.Transactions().ListAccount(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var infos []payments.TransactionInfo
	for _, tx := range txs {
		link := coinpayments.GetCheckoutURL(tx.Key, tx.ID)

		var status payments.TransactionStatus
		switch tx.Status {
		case coinpayments.StatusPending:
			status = payments.TransactionStatusPending
		case coinpayments.StatusReceived:
			status = payments.TransactionStatusPaid
		case coinpayments.StatusCancelled:
			status = payments.TransactionStatusCancelled
		default:
			// unknown
			status = payments.TransactionStatus(tx.Status.String())
		}

		infos = append(infos,
			payments.TransactionInfo{
				ID:        []byte(tx.ID),
				Amount:    *payments.TokenAmountFromBigFloat(&tx.Amount),
				Received:  *payments.TokenAmountFromBigFloat(&tx.Received),
				Address:   tx.Address,
				Status:    status,
				Link:      link,
				ExpiresAt: tx.CreatedAt.Add(tx.Timeout),
				CreatedAt: tx.CreatedAt,
			},
		)
	}

	return infos, nil
}
