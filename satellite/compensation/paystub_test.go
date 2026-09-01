// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

func TestReadAnyPaystubs(t *testing.T) {
	period, err := PeriodFromString("2020-01")
	require.NoError(t, err)

	t.Run("finalized paystubs keep the distributed amount", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, strictcsv.Write(&buf, []Paystub{
			{
				Period:      period,
				NodeID:      testNode1,
				Codes:       Codes{},
				Owed:        currency.NewMicroUnit(100),
				Held:        currency.NewMicroUnit(25),
				Disposed:    currency.NewMicroUnit(10),
				Paid:        currency.NewMicroUnit(110),
				Distributed: currency.NewMicroUnit(110),
			},
		}))

		paystubs, err := ReadAnyPaystubs(buf.Bytes())
		require.NoError(t, err)
		require.Equal(t, []Paystub{
			{
				Period: period,
				NodeID: testNode1,
				// an empty codes column unmarshals to a nil slice
				Codes:       nil,
				Owed:        currency.NewMicroUnit(100),
				Held:        currency.NewMicroUnit(25),
				Disposed:    currency.NewMicroUnit(10),
				Paid:        currency.NewMicroUnit(110),
				Distributed: currency.NewMicroUnit(110),
			},
		}, paystubs)
	})

	t.Run("incomplete paystubs drop the possibly distributed amount", func(t *testing.T) {
		var buf bytes.Buffer
		require.NoError(t, strictcsv.Write(&buf, []IncompletePaystub{
			{
				Period:              period,
				NodeID:              testNode1,
				Codes:               Codes{Bonus, Sanctioned},
				Owed:                currency.NewMicroUnit(100),
				Held:                currency.NewMicroUnit(25),
				Disposed:            currency.NewMicroUnit(10),
				Paid:                currency.NewMicroUnit(110),
				PossiblyDistributed: currency.NewMicroUnit(150),
			},
		}))

		paystubs, err := ReadAnyPaystubs(buf.Bytes())
		require.NoError(t, err)
		require.Equal(t, []Paystub{
			{
				Period: period,
				NodeID: testNode1,
				// Complete drops the bonus code, which the satellites do not support.
				Codes:       Codes{Sanctioned},
				Owed:        currency.NewMicroUnit(100),
				Held:        currency.NewMicroUnit(25),
				Disposed:    currency.NewMicroUnit(10),
				Paid:        currency.NewMicroUnit(110),
				Distributed: currency.Zero,
			},
		}, paystubs)
	})

	t.Run("empty file", func(t *testing.T) {
		_, err := ReadAnyPaystubs(nil)
		require.ErrorContains(t, err, "unable to read CSV headers")
	})

	t.Run("neither format", func(t *testing.T) {
		_, err := ReadAnyPaystubs([]byte("period,node-id\n"))
		require.ErrorContains(t, err, "field headers")
	})
}

func TestPaystubLegalHold(t *testing.T) {
	period, err := PeriodFromString("2020-01")
	require.NoError(t, err)

	paystub := Paystub{
		Period:       period,
		NodeID:       testNode1,
		Codes:        Codes{},
		UsageAtRest:  1,
		UsageGet:     2,
		CompAtRest:   currency.NewMicroUnit(3),
		CompGet:      currency.NewMicroUnit(4),
		SurgePercent: 5,
		Owed:         currency.NewMicroUnit(100),
		Held:         currency.NewMicroUnit(25),
		Disposed:     currency.NewMicroUnit(10),
		Paid:         currency.NewMicroUnit(110),
		Distributed:  currency.NewMicroUnit(110),
	}

	onHold := paystub.LegalHold()

	// the paid and disposed amounts are dropped, so nothing becomes available
	// to the operator and the withheld escrow is left for a later disposal.
	// The rest of the paystub, including owed and held, is kept.
	expected := paystub
	expected.Paid = currency.Zero
	expected.Disposed = currency.Zero
	require.Equal(t, expected, onHold)

	// and the original is untouched
	require.Equal(t, currency.NewMicroUnit(110), paystub.Paid)
	require.Equal(t, currency.NewMicroUnit(10), paystub.Disposed)
}
