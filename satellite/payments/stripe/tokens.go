// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/stripe/stripe-go/v81"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/coinpayments"
)

const (
	// StripeDepositTransactionDescription is the description for Stripe
	// balance transactions representing STORJ deposits.
	StripeDepositTransactionDescription = "STORJ deposit"

	// StripeDepositBonusTransactionDescription is the description for Stripe
	// balance transactions representing bonuses received for STORJ deposits.
	StripeDepositBonusTransactionDescription = "STORJ deposit bonus"
)

// ensure that storjTokens implements payments.StorjTokens.
var _ payments.StorjTokens = (*storjTokens)(nil)

// storjTokens implements payments.StorjTokens.
//
// architecture: Service
type storjTokens struct {
	service *Service
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

		rate, err := tokens.service.db.Transactions().GetLockedRate(ctx, tx.ID)
		if err != nil {
			return nil, err
		}

		infos = append(infos,
			payments.TransactionInfo{
				ID:            []byte(tx.ID),
				Amount:        tx.Amount,
				Received:      tx.Received,
				AmountCents:   convertToCents(rate, tx.Amount),
				ReceivedCents: convertToCents(rate, tx.Received),
				Address:       tx.Address,
				Status:        status,
				Link:          link,
				ExpiresAt:     tx.CreatedAt.Add(tx.Timeout),
				CreatedAt:     tx.CreatedAt,
			},
		)
	}

	return infos, nil
}

// ListDepositBonuses returns all deposit bonuses from Stripe associated with user.
func (tokens *storjTokens) ListDepositBonuses(ctx context.Context, userID uuid.UUID) (_ []payments.DepositBonus, err error) {
	defer mon.Task()(&ctx, userID)(&err)

	cusID, err := tokens.service.db.Customers().GetCustomerID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var bonuses []payments.DepositBonus

	params := &stripe.CustomerParams{Params: stripe.Params{Context: ctx}}
	customer, err := tokens.service.stripeClient.Customers().Get(cusID, params)
	if err != nil {
		return nil, err
	}

	for key, value := range customer.Metadata {
		if !strings.HasPrefix(key, "credit_") {
			continue
		}

		var credit payments.Credit
		err = json.Unmarshal([]byte(value), &credit)
		if err != nil {
			tokens.service.log.Error("Error unmarshaling credit history from Stripe metadata",
				zap.String("Customer ID", cusID),
				zap.String("Metadata Key", key),
				zap.String("Metadata Value", value),
				zap.Error(err),
			)
			continue
		}

		bonuses = append(bonuses,
			payments.DepositBonus{
				TransactionID: payments.TransactionID(credit.TransactionID),
				AmountCents:   credit.Amount,
				Percentage:    10,
				CreatedAt:     credit.Created,
			},
		)
	}

	it := tokens.service.stripeClient.CustomerBalanceTransactions().List(&stripe.CustomerBalanceTransactionListParams{
		ListParams: stripe.ListParams{Context: ctx},
		Customer:   stripe.String(cusID),
	})
	for it.Next() {
		tx := it.CustomerBalanceTransaction()

		if tx.Type != stripe.CustomerBalanceTransactionTypeAdjustment {
			continue
		}

		if tx.Description != StripeDepositBonusTransactionDescription {
			continue
		}

		percentage := int64(10)
		percentageStr, ok := tx.Metadata["percentage"]
		if ok {
			percentage, err = strconv.ParseInt(percentageStr, 10, 64)
			if err != nil {
				return nil, err
			}
		}

		bonuses = append(bonuses,
			payments.DepositBonus{
				TransactionID: []byte(tx.Metadata["txID"]),
				AmountCents:   -tx.Amount,
				Percentage:    percentage,
				CreatedAt:     time.Unix(tx.Created, 0),
			},
		)
	}

	return bonuses, nil
}
