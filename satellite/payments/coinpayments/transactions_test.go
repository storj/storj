// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/payments/coinpayments"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
)

func TestListInfos(t *testing.T) {
	// This test is deliberately skipped as it requires credentials to coinpayments.net
	t.SkipNow()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	payments := coinpayments.NewClient(coinpayments.Credentials{
		PublicKey:  "ask-littleskunk-on-keybase",
		PrivateKey: "ask-littleskunk-on-keybase",
	}).Transactions()

	// verify that bad ids fail
	infos, err := payments.ListInfos(ctx, coinpayments.TransactionIDList{"an_unlikely_id"})
	assert.Error(t, err)
	assert.Len(t, infos, 0)

	// verify that ListInfos can handle more than 25 good ids
	ids := coinpayments.TransactionIDList{}
	for x := 0; x < 27; x++ {
		tx, err := payments.Create(ctx,
			&coinpayments.CreateTX{
				Amount:      decimal.NewFromInt(100),
				CurrencyIn:  currency.StorjToken,
				CurrencyOut: currency.StorjToken,
				BuyerEmail:  "test@test.com",
			},
		)
		ids = append(ids, tx.ID)
		assert.NoError(t, err)
	}
	infos, err = payments.ListInfos(ctx, ids)
	assert.NoError(t, err)
	assert.Len(t, infos, 27)
}

func TestUpdateSameAppliesDoesNotExplode(t *testing.T) {
	satellitedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db satellite.DB) {
		tdb := db.StripeCoinPayments().Transactions()
		assert.NoError(t, tdb.Update(ctx, nil, coinpayments.TransactionIDList{"blah", "blah"}))
		assert.NoError(t, tdb.Update(ctx, nil, coinpayments.TransactionIDList{"blah", "blah"}))
	})
}
