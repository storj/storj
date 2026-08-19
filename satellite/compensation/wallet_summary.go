// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"
	"sort"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

// WalletSummaryRow is one aggregated row per wallet.
type WalletSummaryRow struct {
	Wallet        string `csv:"wallet"`
	Distributable Amount `csv:"distributable"`
	HeldForGE     Amount `csv:"held-for-ge"`
}

// SatelliteReport is one satellite's paired invoice and incompletepaystub CSVs.
// IncompletePaystubs must be from the same generate-invoices → prepare run as
// Invoices (they are joined on node-id).
type SatelliteReport struct {
	Name               string
	Invoices           io.Reader
	IncompletePaystubs io.Reader
}

// zeroWallet is the all-zero Ethereum address; invoices carrying it are
// skipped in wallet-summary.
const zeroWallet = "0x0000000000000000000000000000000000000000"

// SummarizeWallets aggregates one row per wallet across all satellites.
//
// The distributable amount comes from the incompletepaystub's possibly-distributed
// (which respects OFAC zeroing done by prepare). It is counted for every invoice,
// including disqualified ones: prepare still writes a prepayout for them and even
// marks it mandatory, so leaving them out would under-report the funding the run
// needs.
//
// The held-for-GE amount is the escrow still held after this period's activity:
// (TotalHeld + Held) − (TotalDisposed + Disposed). It is dropped for invoices
// carrying the Disqualified code, since those nodes can no longer graceful-exit
// and forfeit the escrow. The code (rather than node-disqualified being set) is
// the right signal: GenerateStatements only zeroes owed/held/disposed, and only
// emits the code, when the disqualification took effect before the period end.
//
// Both Held and Disposed are this period's amounts and are not yet part of
// TotalHeld/TotalDisposed, so the input must come from a generate-invoices run
// made before record-period wrote the period's paystubs. Re-running
// generate-invoices for an already-recorded period double-counts the escrow,
// because record-period replaces the paystub on (period, node-id) and the totals
// then already include Held/Disposed.
//
// Invoices with an empty or all-zero wallet are skipped, as are wallets whose
// aggregated distributable and held-for-GE are both zero.
func SummarizeWallets(reports []SatelliteReport, out io.Writer) error {
	sums := map[string]*walletAcc{}

	for _, report := range reports {
		invoices, err := ReadInvoicesLenient(report.Invoices)
		if err != nil {
			return errs.New("satellite %q: reading invoices: %w", report.Name, err)
		}
		ipaystubs, err := ReadIncompletePaystubs(report.IncompletePaystubs)
		if err != nil {
			return errs.New("satellite %q: reading incompletepaystubs: %w", report.Name, err)
		}

		possiblyDistributedByNode := make(map[NodeID]currency.MicroUnit, len(ipaystubs))
		for _, ip := range ipaystubs {
			possiblyDistributedByNode[ip.NodeID] = ip.PossiblyDistributed
		}

		for _, inv := range invoices {
			wallet := strings.ToLower(strings.TrimSpace(inv.NodeWallet))
			if wallet == "" || wallet == zeroWallet {
				continue
			}
			possiblyDistributed, ok := possiblyDistributedByNode[inv.NodeID]
			if !ok {
				return errs.New("satellite %q: node %s in invoices but not in incompletepaystubs", report.Name, inv.NodeID)
			}
			acc, ok := sums[wallet]
			if !ok {
				acc = &walletAcc{}
				sums[wallet] = acc
			}
			acc.distributable += possiblyDistributed.Value()
			if !containsCode(inv.Codes, Disqualified) {
				acc.heldForGE += (inv.TotalHeld.Value() + inv.Held.Value()) - (inv.TotalDisposed.Value() + inv.Disposed.Value())
			}
		}
	}

	rows := make([]WalletSummaryRow, 0, len(sums))
	for wallet, acc := range sums {
		if acc.distributable == 0 && acc.heldForGE == 0 {
			continue
		}
		rows = append(rows, WalletSummaryRow{
			Wallet:        wallet,
			Distributable: Amount(currency.NewMicroUnit(acc.distributable)),
			HeldForGE:     Amount(currency.NewMicroUnit(acc.heldForGE)),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Wallet < rows[j].Wallet })

	return strictcsv.Write(out, rows)
}

type walletAcc struct {
	distributable int64
	heldForGE     int64
}
