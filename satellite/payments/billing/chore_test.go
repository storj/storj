// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package billing_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/payments/monetary"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestChore(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		logger := zaptest.NewLogger(t)

		userID := testrand.UUID()
		billingDB := db.Billing()

		var batch, runningBatch []billing.Transaction
		paymentType := struct{ mockPayment }{}
		paymentType.mockSource = func() string { return "mockPaymentService" }
		paymentType.mockType = func() billing.TransactionType { return billing.TransactionTypeCredit }
		paymentType.mockGetNewTransactions = func(ctx context.Context,
			lastTransactionTime time.Time, metadata []byte) ([]billing.Transaction, error) {
			return batch, nil
		}

		chore := billing.NewChore(logger, []billing.PaymentType{paymentType}, billingDB, time.Minute, false)
		ctx.Go(func() error {
			return chore.Run(ctx)
		})
		defer ctx.Check(chore.Close)

		chore.TransactionCycle.Pause()

		batch = createBatch(t, userID, 0, 0)
		runningBatch = append(runningBatch, batch...)

		chore.TransactionCycle.TriggerWait()
		chore.TransactionCycle.Pause()

		transactions, err := billingDB.List(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, len(runningBatch), len(transactions))
		for _, act := range transactions {
			for _, exp := range runningBatch {
				if act.ID == exp.ID {
					compareTransactions(t, exp, act)
					break
				}
			}
		}

		batch = createBatch(t, userID, 3, 4)
		runningBatch = append(runningBatch, batch...)

		chore.TransactionCycle.TriggerWait()
		chore.TransactionCycle.Pause()

		transactions, err = billingDB.List(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, len(runningBatch), len(transactions))
		for _, act := range transactions {
			for _, exp := range runningBatch {
				if act.ID == exp.ID {
					compareTransactions(t, exp, act)
					break
				}
			}
		}
	})
}

func createBatch(t *testing.T, userID uuid.UUID, blockNumber int64, logIndex int) []billing.Transaction {
	tenUSD := monetary.AmountFromBaseUnits(1000, monetary.USDollars)
	twentyUSD := monetary.AmountFromBaseUnits(2000, monetary.USDollars)
	thirtyUSD := monetary.AmountFromBaseUnits(3000, monetary.USDollars)

	address, err := blockchain.BytesToAddress(testrand.Bytes(20))
	require.NoError(t, err)

	metadata, err := json.Marshal(map[string]interface{}{
		"ReferenceID": "some invoice ID",
		"Wallet":      address.Hex(),
		"BlockNumber": blockNumber,
		"LogIndex":    logIndex,
	})
	require.NoError(t, err)

	credit10TX := billing.Transaction{
		UserID:      userID,
		Amount:      tenUSD,
		Description: "credit from mock payment",
		Source:      "mockPaymentService",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second),
		CreatedAt:   time.Now(),
	}

	credit20TX := billing.Transaction{
		UserID:      userID,
		Amount:      twentyUSD,
		Description: "credit from mock payment",
		Source:      "mockPaymentService",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second * 2),
		CreatedAt:   time.Now(),
	}

	credit30TX := billing.Transaction{
		UserID:      userID,
		Amount:      thirtyUSD,
		Description: "credit from mock payment",
		Source:      "mockPaymentService",
		Status:      billing.TransactionStatusCompleted,
		Type:        billing.TransactionTypeCredit,
		Metadata:    metadata,
		Timestamp:   time.Now().Add(time.Second * 4),
		CreatedAt:   time.Now(),
	}
	return []billing.Transaction{credit10TX, credit20TX, credit30TX}
}

// setup mock payment type.
var _ billing.PaymentType = (*mockPayment)(nil)

type mockPayment struct {
	mockSource             func() string
	mockType               func() billing.TransactionType
	mockGetNewTransactions func(ctx context.Context, lastTransactionTime time.Time, metadata []byte) ([]billing.Transaction, error)
}

func (t mockPayment) Source() string                { return t.mockSource() }
func (t mockPayment) Type() billing.TransactionType { return t.mockType() }
func (t mockPayment) GetNewTransactions(ctx context.Context, lastTransactionTime time.Time, metadata []byte) ([]billing.Transaction, error) {
	return t.mockGetNewTransactions(ctx, lastTransactionTime, metadata)
}
