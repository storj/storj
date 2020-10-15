// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/common/testcontext"
)

func TestListInfos(t *testing.T) {
	// This test is deliberately skipped as it requires credentials to coinpayments.net
	t.SkipNow()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	payments := NewClient(Credentials{
		PublicKey:  "ask-littleskunk-on-keybase",
		PrivateKey: "ask-littleskunk-on-keybase",
	}).Transactions()

	// verify that bad ids fail
	infos, err := payments.ListInfos(ctx, TransactionIDList{"an_unlikely_id"})
	assert.Error(t, err)
	assert.Len(t, infos, 0)

	// verify that ListInfos can handle more than 25 good ids
	ids := TransactionIDList{}
	for x := 0; x < 27; x++ {
		tx, err := payments.Create(ctx,
			&CreateTX{
				Amount:      *big.NewFloat(100),
				CurrencyIn:  CurrencySTORJ,
				CurrencyOut: CurrencySTORJ,
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
