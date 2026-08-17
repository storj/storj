// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"

	"storj.io/storj/shared/strictcsv"
)

// Receipt describes a payment that was executed against a wallet.
type Receipt struct {
	Wallet    string `csv:"wallet"`
	Amount    Amount `csv:"amount"`
	TxHash    string `csv:"txhash"`
	Mechanism string `csv:"mechanism"`
}

// ReadReceipts reads a collection of Receipts in CSV form.
func ReadReceipts(r io.Reader) ([]Receipt, error) {
	var receipts []Receipt
	if err := strictcsv.Read(r, &receipts); err != nil {
		return nil, err
	}
	return receipts, nil
}
