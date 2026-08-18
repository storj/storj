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

// SummarizeWallets aggregates one row per wallet across all satellites.
//
// The distributable amount comes from the incompletepaystub's possibly-distributed
// (which respects OFAC zeroing done by prepare). The held-for-GE amount comes
// from the invoice's TotalHeld − TotalDisposed and is zeroed for disqualified
// nodes (they can't GE, so the escrow is forfeited).
//
// Wallets are lowercased for grouping; invoices with an empty wallet are skipped.
func SummarizeWallets(reports []SatelliteReport, out io.Writer) error {
	sums := map[string]*walletAcc{}

	for _, report := range reports {
		invoices, err := ReadInvoices(report.Invoices)
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
			if wallet == "" {
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
			if inv.NodeDisqualified == nil {
				acc.heldForGE += inv.TotalHeld.Value() - inv.TotalDisposed.Value()
			}
		}
	}

	rows := make([]WalletSummaryRow, 0, len(sums))
	for wallet, acc := range sums {
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
