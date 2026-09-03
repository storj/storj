// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

const testWalletSummaryHeader = "wallet,distributable,held-for-ge"

// summarize renders invoices and their matching incompletepaystubs into CSVs and
// runs SummarizeWallets over them, returning the data rows of the output.
func summarize(t *testing.T, invoices []Invoice, ipaystubs []IncompletePaystub) []string {
	t.Helper()

	invoicesCSV := new(bytes.Buffer)
	require.NoError(t, WriteInvoices(invoicesCSV, invoices))

	ipaystubsCSV := new(bytes.Buffer)
	require.NoError(t, strictcsv.Write(ipaystubsCSV, ipaystubs))

	out := new(bytes.Buffer)
	require.NoError(t, SummarizeWallets([]SatelliteReport{{
		Name:               "test",
		Invoices:           invoicesCSV,
		IncompletePaystubs: ipaystubsCSV,
	}}, out))

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	require.Equal(t, testWalletSummaryHeader, lines[0])
	return lines[1:]
}

func TestSummarizeWallets(t *testing.T) {
	period := Period{Year: 2026, Month: 8}
	micro := currency.NewMicroUnit

	// invoice builds an invoice plus the incompletepaystub prepare would emit for
	// it. possiblyDistributed is what prepare computed, which is what the summary
	// must report as distributable.
	invoice := func(wallet string, codes Codes, held, disposed, totalHeld, totalDisposed, possiblyDistributed int64) (Invoice, IncompletePaystub) {
		nodeID := NodeID(testrand.NodeID())
		return Invoice{
			Period:        period,
			NodeID:        nodeID,
			NodeWallet:    wallet,
			Codes:         codes,
			Held:          micro(held),
			Disposed:      micro(disposed),
			TotalHeld:     micro(totalHeld),
			TotalDisposed: micro(totalDisposed),
		}, IncompletePaystub{
			Period:              period,
			NodeID:              nodeID,
			Codes:               codes,
			PossiblyDistributed: micro(possiblyDistributed),
		}
	}

	t.Run("held-for-ge includes this period's held and disposed", func(t *testing.T) {
		// The period's own held/disposed are not in total-held/total-disposed yet,
		// so escrow after the run is (500+100) - (200+50) = 350.
		inv, ips := invoice("0xAAA", nil, 100, 50, 500, 200, 1000)
		require.Equal(t, []string{"0xaaa,0.001000,0.000350"},
			summarize(t, []Invoice{inv}, []IncompletePaystub{ips}))
	})

	t.Run("disqualified node keeps distributable but forfeits held-for-ge", func(t *testing.T) {
		// prepare still writes a (mandatory) prepayout for a disqualified node, so
		// dropping it here would under-report the funding the run needs.
		inv, ips := invoice("0xBBB", Codes{Disqualified}, 0, 0, 500, 200, 1000)
		require.Equal(t, []string{"0xbbb,0.001000,0.000000"},
			summarize(t, []Invoice{inv}, []IncompletePaystub{ips}))
	})

	t.Run("node disqualified after the period keeps held-for-ge", func(t *testing.T) {
		// GenerateStatements only zeroes owed/held/disposed, and only emits the
		// Disqualified code, when the disqualification took effect before the
		// period end. node-disqualified alone must not drop the escrow.
		inv, ips := invoice("0xCCC", nil, 100, 50, 500, 200, 1000)
		disqualified := UTCDate(period.EndDateExclusive())
		inv.NodeDisqualified = &disqualified
		require.Equal(t, []string{"0xccc,0.001000,0.000350"},
			summarize(t, []Invoice{inv}, []IncompletePaystub{ips}))
	})

	t.Run("invoices are grouped per wallet, case insensitively", func(t *testing.T) {
		invA, ipsA := invoice("0xDDD", nil, 100, 0, 0, 0, 1000)
		invB, ipsB := invoice("0xddd", nil, 200, 0, 0, 0, 2000)
		require.Equal(t, []string{"0xddd,0.003000,0.000300"},
			summarize(t, []Invoice{invA, invB}, []IncompletePaystub{ipsA, ipsB}))
	})

	t.Run("empty, zero and all-zero wallets are skipped", func(t *testing.T) {
		invEmpty, ipsEmpty := invoice("", nil, 100, 0, 0, 0, 1000)
		invZero, ipsZero := invoice(zeroWallet, nil, 100, 0, 0, 0, 1000)
		// a wallet whose aggregate is entirely zero is not worth a row either
		invNothing, ipsNothing := invoice("0xEEE", nil, 0, 0, 0, 0, 0)
		require.Empty(t, summarize(t,
			[]Invoice{invEmpty, invZero, invNothing},
			[]IncompletePaystub{ipsEmpty, ipsZero, ipsNothing}))
	})

	t.Run("invoice without a matching incompletepaystub is an error", func(t *testing.T) {
		inv, _ := invoice("0xFFF", nil, 0, 0, 0, 0, 0)

		invoicesCSV := new(bytes.Buffer)
		require.NoError(t, WriteInvoices(invoicesCSV, []Invoice{inv}))
		ipaystubsCSV := new(bytes.Buffer)
		require.NoError(t, strictcsv.Write(ipaystubsCSV, []IncompletePaystub{}))

		err := SummarizeWallets([]SatelliteReport{{
			Name:               "test",
			Invoices:           invoicesCSV,
			IncompletePaystubs: ipaystubsCSV,
		}}, new(bytes.Buffer))
		require.ErrorContains(t, err, "in invoices but not in incompletepaystubs")
	})
}
