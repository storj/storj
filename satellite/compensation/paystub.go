// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"
	"os"

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
