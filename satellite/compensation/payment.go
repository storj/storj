// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"
	"os"

	"storj.io/storj/pkg/strictcsv"
	"storj.io/storj/private/currency"
)

type Payment struct {
	Period  Period             `csv:"period"`
	NodeID  NodeID             `csv:"node-id"`
	Amount  currency.MicroUnit `csv:"amount"`
	Receipt *string            `csv:"receipt"`
	Notes   *string            `csv:"notes"`
}

func LoadPayments(path string) ([]Payment, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() { _ = f.Close() }()
	return ReadPayments(f)
}

func ReadPayments(r io.Reader) ([]Payment, error) {
	var payments []Payment
	if err := strictcsv.Read(r, &payments); err != nil {
		return nil, err
	}
	return payments, nil
}

func WritePayments(w io.Writer, payments []Payment) error {
	return strictcsv.Write(w, payments)
}
