// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/strictcsv"
)

func TestParseSatelliteFiles(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		files, err := parseSatelliteFiles("paystubs", "ap1:2026-07-ap1-incompletepaystubs.csv, us1:/tmp/us1.csv")
		require.NoError(t, err)
		require.Equal(t, []satelliteFile{
			{Name: "ap1", Path: "2026-07-ap1-incompletepaystubs.csv"},
			{Name: "us1", Path: "/tmp/us1.csv"},
		}, files)
	})

	for _, tt := range []struct {
		name     string
		value    string
		errorMsg string
	}{
		{"empty", "", "--paystubs is empty"},
		{"only separators", " , ", "--paystubs is empty"},
		{"missing satellite", "a.csv", `--paystubs entry "a.csv" is not in the <satellite>:<path> form`},
		{"missing path", "us1:", `--paystubs entry "us1:" is not in the <satellite>:<path> form`},
		{"missing name", ":a.csv", `--paystubs entry ":a.csv" is not in the <satellite>:<path> form`},
		{"duplicate satellite", "us1:a.csv,us1:b.csv", `--paystubs has more than one entry for satellite "us1"`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseSatelliteFiles("paystubs", tt.value)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errorMsg)
		})
	}
}

func TestSatelliteFilesFor(t *testing.T) {
	paystubs := []satelliteFile{
		{Name: "ap1", Path: "reports/2026-07-ap1-incompletepaystubs.csv"},
		{Name: "us1", Path: "reports/2026-07-us1-incompletepaystubs.csv"},
	}

	t.Run("derived from the paystubs", func(t *testing.T) {
		files, err := satelliteFilesFor("invoices", "", paystubs, "invoices.csv")
		require.NoError(t, err)
		require.Equal(t, []satelliteFile{
			{Name: "ap1", Path: "reports/2026-07-ap1-invoices.csv"},
			{Name: "us1", Path: "reports/2026-07-us1-invoices.csv"},
		}, files)
	})

	t.Run("cannot be derived from another name", func(t *testing.T) {
		_, err := satelliteFilesFor("invoices", "", []satelliteFile{{Name: "us1", Path: "paystubs.csv"}}, "invoices.csv")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot derive the invoices path of satellite \"us1\"")
	})

	// The explicit list is matched to the paystubs by name, so the two flags do
	// not have to be written in the same order.
	t.Run("explicit list follows the paystub order", func(t *testing.T) {
		files, err := satelliteFilesFor("payments", "us1:u.csv,ap1:a.csv", paystubs, "payments.csv")
		require.NoError(t, err)
		require.Equal(t, []satelliteFile{
			{Name: "ap1", Path: "a.csv"},
			{Name: "us1", Path: "u.csv"},
		}, files)
	})

	t.Run("explicit list must cover the same satellites", func(t *testing.T) {
		_, err := satelliteFilesFor("payments", "us1:u.csv,eu1:e.csv", paystubs, "payments.csv")
		require.Error(t, err)
		require.Contains(t, err.Error(), `--payments has no entry for satellite "ap1"`)

		_, err = satelliteFilesFor("payments", "us1:u.csv", paystubs, "payments.csv")
		require.Error(t, err)
		require.Contains(t, err.Error(), "--payments lists 1 satellites but --paystubs lists 2")
	})
}

func TestGeneratePaymentsRun(t *testing.T) {
	period, err := compensation.PeriodFromString("2026-07")
	require.NoError(t, err)

	dir := t.TempDir()
	write := func(name string, rows any) string {
		path := filepath.Join(dir, name)
		file, err := os.Create(path)
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()
		require.NoError(t, strictcsv.Write(file, rows))
		return path
	}

	node1 := compensation.NodeID(storj.NodeID{1})
	node2 := compensation.NodeID(storj.NodeID{2})
	wallet := "0x0000000000000000000000000000000000000001"

	invoice := func(nodeID compensation.NodeID) compensation.Invoice {
		return compensation.Invoice{Period: period, NodeID: nodeID, NodeWallet: wallet}
	}
	ipaystub := func(nodeID compensation.NodeID, amount int64) compensation.IncompletePaystub {
		return compensation.IncompletePaystub{
			Period:              period,
			NodeID:              nodeID,
			Codes:               compensation.Codes{},
			PossiblyDistributed: currency.NewMicroUnit(amount),
		}
	}

	// The invoice paths are not passed in, so they have to be derived from the
	// paystub file names.
	write("2026-07-ap1-invoices.csv", []compensation.Invoice{invoice(node1)})
	write("2026-07-us1-invoices.csv", []compensation.Invoice{invoice(node2)})
	ap1Paystubs := write("2026-07-ap1-incompletepaystubs.csv", []compensation.IncompletePaystub{ipaystub(node1, 1000000)})
	us1Paystubs := write("2026-07-us1-incompletepaystubs.csv", []compensation.IncompletePaystub{ipaystub(node2, 2500000)})
	receipts := write("combined-payouts-all-receipts.csv", []compensation.Receipt{{
		Wallet:    wallet,
		Amount:    compensation.Amount(currency.NewMicroUnit(3500000)),
		TxHash:    "0xdeadbeef",
		Mechanism: "eth",
	}})

	summaryPath := filepath.Join(dir, "summary.txt")
	cmd := NewGeneratePayments(zaptest.NewLogger(t), &GeneratePaymentsConfig{
		Paystubs:   "ap1:" + ap1Paystubs + ",us1:" + us1Paystubs,
		Receipts:   receipts,
		SummaryOut: summaryPath,
	}, &modular.StopTrigger{Cancel: func() {}})
	require.NoError(t, cmd.Run(context.Background()))

	for _, tt := range []struct {
		payments string
		paystubs string
		nodeID   compensation.NodeID
		amount   int64
	}{
		{payments: "2026-07-ap1-payments.csv", paystubs: "2026-07-ap1-paystubs.csv", nodeID: node1, amount: 1000000},
		{payments: "2026-07-us1-payments.csv", paystubs: "2026-07-us1-paystubs.csv", nodeID: node2, amount: 2500000},
	} {
		payments, err := compensation.LoadPayments(filepath.Join(dir, tt.payments))
		require.NoError(t, err)
		require.Len(t, payments, 1)
		require.Equal(t, tt.nodeID, payments[0].NodeID)
		require.Equal(t, tt.amount, payments[0].Amount.Value())
		require.Equal(t, period, payments[0].Period)
		require.NotNil(t, payments[0].Receipt)
		require.Equal(t, "eth:0xdeadbeef", *payments[0].Receipt)

		// The paystubs are the other half of the output: they carry the amount
		// the payment moved as distributed, so the next period does not carry it
		// over and pay it again. LoadPaystubs is what record-period reads them
		// with, and unlike LoadAnyPaystubs it keeps that amount.
		paystubs, err := compensation.LoadPaystubs(filepath.Join(dir, tt.paystubs))
		require.NoError(t, err)
		require.Len(t, paystubs, 1)
		require.Equal(t, tt.nodeID, paystubs[0].NodeID)
		require.Equal(t, period, paystubs[0].Period)
		require.Equal(t, tt.amount, paystubs[0].Distributed.Value())
	}

	summary, err := os.ReadFile(summaryPath)
	require.NoError(t, err)
	require.Equal(t, ""+
		"SATELLITE  NODES  PAYMENTS  POSSIBLY-DISTRIBUTED  PAID      UNATTRIBUTED\n"+
		"ap1        1      1         1.000000              1.000000  0.000000\n"+
		"us1        1      1         2.500000              2.500000  0.000000\n"+
		"TOTAL      2      2         3.500000              3.500000  0.000000\n"+
		"\nunpaid amounts max: 0.000000, sum: 0.000000, over 0 wallets\n",
		string(summary))
}

// Nothing may be left behind when the payouts cannot be attributed, otherwise a
// partial payments file could be recorded as if it described the whole payout.
func TestGeneratePaymentsRunWritesNothingOnFailure(t *testing.T) {
	period, err := compensation.PeriodFromString("2026-07")
	require.NoError(t, err)

	dir := t.TempDir()
	write := func(name string, rows any) string {
		path := filepath.Join(dir, name)
		file, err := os.Create(path)
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()
		require.NoError(t, strictcsv.Write(file, rows))
		return path
	}

	node := compensation.NodeID(storj.NodeID{1})
	wallet := "0x0000000000000000000000000000000000000001"

	write("2026-07-us1-invoices.csv", []compensation.Invoice{{Period: period, NodeID: node, NodeWallet: wallet}})
	paystubs := write("2026-07-us1-incompletepaystubs.csv", []compensation.IncompletePaystub{{
		Period:              period,
		NodeID:              node,
		Codes:               compensation.Codes{},
		PossiblyDistributed: currency.NewMicroUnit(1000000),
	}})
	// The transferred amount does not match what the paystub says was owed. It
	// transferred more, which is the direction --continue covers: the node is
	// still credited exactly what it is owed, and only the surplus is
	// unexplained.
	receipts := write("receipts.csv", []compensation.Receipt{{
		Wallet:    wallet,
		Amount:    compensation.Amount(currency.NewMicroUnit(1100000)),
		TxHash:    "0xdeadbeef",
		Mechanism: "eth",
	}})

	config := &GeneratePaymentsConfig{
		Paystubs:   "us1:" + paystubs,
		Receipts:   receipts,
		SummaryOut: filepath.Join(dir, "summary.txt"),
	}
	cmd := NewGeneratePayments(zaptest.NewLogger(t), config, &modular.StopTrigger{Cancel: func() {}})

	err = cmd.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "the payouts cannot be attributed to the paystubs")

	for _, name := range []string{
		"2026-07-us1-payments.csv", "2026-07-us1-payments.csv.tmp",
		"2026-07-us1-paystubs.csv", "2026-07-us1-paystubs.csv.tmp",
		"summary.txt", "summary.txt.tmp",
	} {
		_, err := os.Stat(filepath.Join(dir, name))
		require.True(t, os.IsNotExist(err), "%q must not be written", name)
	}

	// With --continue the mismatch is only reported, and the payment is written
	// with the amount the paystub says the node is owed.
	config.Continue = true
	require.NoError(t, cmd.Run(context.Background()))

	payments, err := compensation.LoadPayments(filepath.Join(dir, "2026-07-us1-payments.csv"))
	require.NoError(t, err)
	require.Len(t, payments, 1)
	require.Equal(t, int64(1000000), payments[0].Amount.Value())

	// A receipt that transferred less than the node is owed is the other
	// direction, and --continue does not write it off: the node would be credited
	// the full amount as distributed, so the difference stops being owed without
	// ever having been paid.
	write("receipts.csv", []compensation.Receipt{{
		Wallet:    wallet,
		Amount:    compensation.Amount(currency.NewMicroUnit(999999)),
		TxHash:    "0xdeadbeef",
		Mechanism: "eth",
	}})

	err = cmd.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "transferred less than the nodes are owed, which --continue does not write off")
}

// Two outputs writing to one path would each be created from offset zero and
// renamed over one another, leaving one interleaved file in place of both.
func TestGeneratePaymentsRunRejectsCollidingOutputs(t *testing.T) {
	period, err := compensation.PeriodFromString("2026-07")
	require.NoError(t, err)

	dir := t.TempDir()
	write := func(name string, rows any) string {
		path := filepath.Join(dir, name)
		file, err := os.Create(path)
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()
		require.NoError(t, strictcsv.Write(file, rows))
		return path
	}

	node := compensation.NodeID(storj.NodeID{1})
	wallet := "0x0000000000000000000000000000000000000001"

	write("2026-07-us1-invoices.csv", []compensation.Invoice{{Period: period, NodeID: node, NodeWallet: wallet}})
	paystubs := write("2026-07-us1-incompletepaystubs.csv", []compensation.IncompletePaystub{{
		Period:              period,
		NodeID:              node,
		Codes:               compensation.Codes{},
		PossiblyDistributed: currency.NewMicroUnit(1000000),
	}})
	receipts := write("receipts.csv", []compensation.Receipt{{
		Wallet:    wallet,
		Amount:    compensation.Amount(currency.NewMicroUnit(1000000)),
		TxHash:    "0xdeadbeef",
		Mechanism: "eth",
	}})

	collision := filepath.Join(dir, "out.csv")
	config := &GeneratePaymentsConfig{
		Paystubs:    "us1:" + paystubs,
		Receipts:    receipts,
		Payments:    "us1:" + collision,
		PaystubsOut: "us1:" + collision,
		SummaryOut:  filepath.Join(dir, "summary.txt"),
	}
	cmd := NewGeneratePayments(zaptest.NewLogger(t), config, &modular.StopTrigger{Cancel: func() {}})

	err = cmd.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "is used more than once")

	// The run must not have started, so neither the output nor its temporary
	// file may be left behind.
	for _, name := range []string{"out.csv", "out.csv.tmp", "summary.txt", "summary.txt.tmp"} {
		_, err := os.Stat(filepath.Join(dir, name))
		require.True(t, os.IsNotExist(err), "%q must not be written", name)
	}
}
