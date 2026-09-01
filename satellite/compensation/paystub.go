// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"encoding/csv"
	"io"
	"os"
	"slices"

	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

// Paystub contains the basic information about a payment that is to be made.
type Paystub struct {
	Period         Period             `csv:"period"`
	NodeID         NodeID             `csv:"node-id"`
	Codes          Codes              `csv:"codes"`
	UsageAtRest    float64            `csv:"usage-at-rest"`
	UsageGet       int64              `csv:"usage-get"`
	UsagePut       int64              `csv:"usage-put"`
	UsageGetRepair int64              `csv:"usage-get-repair"`
	UsagePutRepair int64              `csv:"usage-put-repair"`
	UsageGetAudit  int64              `csv:"usage-get-audit"`
	CompAtRest     currency.MicroUnit `csv:"comp-at-rest"`
	CompGet        currency.MicroUnit `csv:"comp-get"`
	CompPut        currency.MicroUnit `csv:"comp-put"`
	CompGetRepair  currency.MicroUnit `csv:"comp-get-repair"`
	CompPutRepair  currency.MicroUnit `csv:"comp-put-repair"`
	CompGetAudit   currency.MicroUnit `csv:"comp-get-audit"`
	SurgePercent   int64              `csv:"surge-percent"`
	Owed           currency.MicroUnit `csv:"owed"`
	Held           currency.MicroUnit `csv:"held"`
	Disposed       currency.MicroUnit `csv:"disposed"`
	Paid           currency.MicroUnit `csv:"paid"`
	Distributed    currency.MicroUnit `csv:"distributed"`
}

// LoadPaystubs loads a collection of Paystubs in CSV form from the provided file.
func LoadPaystubs(path string) ([]Paystub, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { _ = f.Close() }()
	return ReadPaystubs(f)
}

// ReadPaystubs reads a collection of Paystubs in CSV form.
func ReadPaystubs(r io.Reader) ([]Paystub, error) {
	var paystubs []Paystub
	if err := strictcsv.Read(r, &paystubs); err != nil {
		return nil, err
	}
	return paystubs, nil
}

// possiblyDistributedHeader is the payout column of the incomplete paystubs
// written by Prepare. The finalized paystubs call the same column "distributed",
// which is what tells the two formats apart.
const possiblyDistributedHeader = "possibly-distributed"

// LoadAnyPaystubs loads paystubs in either CSV form from the provided file.
// See ReadAnyPaystubs.
func LoadAnyPaystubs(path string) ([]Paystub, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return ReadAnyPaystubs(data)
}

// ReadAnyPaystubs reads a collection of Paystubs from a CSV in either the
// finalized paystubs form or the incomplete paystubs form written by Prepare.
// The forms differ only in their payout column: "distributed" for the former,
// "possibly-distributed" for the latter.
//
// The payout column is only carried over from the finalized form. There a
// non-zero distributed amount is written by Finalize only for a paystub whose
// wallet had a matching receipt, and reconcileReceipts additionally checks that
// the amounts credited to a wallet add up to what its receipt says was
// transferred, so the amount is backed by proof that the money moved. An
// incomplete paystub only carries the amount that would be distributed if the
// payout were executed, so those are returned with a zero distributed amount.
//
// It takes the bytes rather than an io.Reader because the format is decided
// from the header, so the input has to be read twice.
func ReadAnyPaystubs(data []byte) ([]Paystub, error) {
	headers, err := csv.NewReader(bytes.NewReader(data)).Read()
	if err != nil {
		return nil, Error.New("unable to read CSV headers: %v", err)
	}

	if slices.Contains(headers, possiblyDistributedHeader) {
		ipaystubs, err := ReadIncompletePaystubs(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		paystubs := make([]Paystub, 0, len(ipaystubs))
		for _, ipaystub := range ipaystubs {
			paystubs = append(paystubs, ipaystub.Complete(currency.Zero))
		}
		return paystubs, nil
	}

	return ReadPaystubs(bytes.NewReader(data))
}

// LegalHold returns a copy of the paystub with the paid and disposed amounts
// zeroed out. `paid` is what makes the money available to the operator, so a
// zero paid withholds it.
//
// `disposed` has to go with it. It is the portion of the withheld escrow that
// was released into `paid` this period, and AllNodeStatements caps future
// disposals with disposed = PercentOf(TotalHeld, disposePercent) -
// TotalDisposed, so recording a disposal next to a zero paid would advance
// TotalDisposed for money the node never received: the escrow would never be
// disposed again, and with paid=0 there is no TotalPaid - TotalDistributed
// carry-over to pay it either. Zeroing it instead leaves the escrow untouched,
// so it is disposed normally once the hold is lifted.
//
// The rest of the paystub is kept as it is, so it still records the usage, what
// the node earned in the period, and what was held. `owed` in particular is
// left alone: it is the record of what the node was owed before the hold, and
// no total is aggregated from it (QueryTotalAmounts sums held, disposed, paid
// and distributed only).
func (paystub Paystub) LegalHold() Paystub {
	paystub.Paid = currency.Zero
	paystub.Disposed = currency.Zero
	return paystub
}
