// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

var (
	testNode1 = NodeID(storj.NodeID{1})
	testNode2 = NodeID(storj.NodeID{2})
	testNode3 = NodeID(storj.NodeID{3})

	testWallet1 = "0x0000000000000000000000000000000000000001"
	testWallet2 = "0x0000000000000000000000000000000000000002"
)

func TestFinalize(t *testing.T) {
	period, err := PeriodFromString("2020-01")
	require.NoError(t, err)

	invoice := func(nodeID NodeID, wallet string, features ...string) Invoice {
		return Invoice{
			Period:             period,
			NodeID:             nodeID,
			NodeWallet:         wallet,
			NodeWalletFeatures: features,
		}
	}

	ipaystub := func(nodeID NodeID, possiblyDistributed int64) IncompletePaystub {
		return IncompletePaystub{
			Period:              period,
			NodeID:              nodeID,
			Codes:               Codes{},
			Owed:                currency.NewMicroUnit(possiblyDistributed),
			Paid:                currency.NewMicroUnit(possiblyDistributed),
			PossiblyDistributed: currency.NewMicroUnit(possiblyDistributed),
		}
	}

	receipt := func(wallet string, amount int64, mechanism, txHash string) Receipt {
		return Receipt{
			Wallet:    wallet,
			Amount:    Amount(currency.NewMicroUnit(amount)),
			TxHash:    txHash,
			Mechanism: mechanism,
		}
	}

	// payment describes the expected payment for a node.
	type payment struct {
		nodeID  NodeID
		amount  int64
		receipt string
	}

	for _, tt := range []struct {
		name        string
		invoices    []Invoice
		ipaystubs   []IncompletePaystub
		receipts    []Receipt
		allowUnpaid bool

		payments    []payment
		distributed map[NodeID]int64
		err         string
	}{
		{
			name: "nothing to finalize",
		},
		{
			name:      "wallet matched",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts:  []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000},
		},
		{
			name: "multiple nodes share a wallet",
			invoices: []Invoice{
				invoice(testNode1, testWallet1),
				invoice(testNode2, testWallet1),
			},
			ipaystubs: []IncompletePaystub{
				ipaystub(testNode1, 1000000),
				ipaystub(testNode2, 2500000),
			},
			receipts: []Receipt{receipt(testWallet1, 3500000, "eth", "0xdeadbeef")},
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"},
				{nodeID: testNode2, amount: 2500000, receipt: "eth:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000, testNode2: 2500000},
		},
		{
			name:      "zero payout is completed without a payment",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 0)},
			// The node is not paid out, so the payout tool produces no receipt for
			// its wallet.
			distributed: map[NodeID]int64{testNode1: 0},
		},
		{
			name: "node without a receipt is carried over to the next period",
			invoices: []Invoice{
				invoice(testNode1, testWallet1),
				invoice(testNode2, testWallet2),
			},
			ipaystubs: []IncompletePaystub{
				ipaystub(testNode1, 1000000),
				// testNode2 was below the payout threshold, so its wallet was not paid.
				ipaystub(testNode2, 10),
			},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000, testNode2: 0},
		},
		{
			name: "too much of the payout has no receipt",
			invoices: []Invoice{
				invoice(testNode1, testWallet1),
				invoice(testNode2, testWallet2),
			},
			ipaystubs: []IncompletePaystub{
				ipaystub(testNode1, 1000000),
				// A fifth of the payout, well above the 5% the test allows.
				ipaystub(testNode2, 250000),
			},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			err:      "refusing to write payouts: 1 nodes (0.250000 of 1.250000) have no receipt, which is more than 5% of the payout (use AllowUnpaid to override)",
		},
		{
			name: "too much of the payout has no receipt, overridden",
			invoices: []Invoice{
				invoice(testNode1, testWallet1),
				invoice(testNode2, testWallet2),
			},
			ipaystubs: []IncompletePaystub{
				ipaystub(testNode1, 1000000),
				ipaystub(testNode2, 250000),
			},
			receipts:    []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			allowUnpaid: true,
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000, testNode2: 0},
		},
		{
			name:      "empty receipts file with a non-zero payout is rejected",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			err:       "refusing to write payouts: the receipts file is empty but 1 nodes (1.000000) are owed a payout (use AllowUnpaid to override)",
		},
		{
			name:        "empty receipts file with a non-zero payout, overridden",
			invoices:    []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs:   []IncompletePaystub{ipaystub(testNode1, 1000000)},
			allowUnpaid: true,
			distributed: map[NodeID]int64{testNode1: 0},
		},
		{
			name:      "wallet address casing is ignored",
			invoices:  []Invoice{invoice(testNode1, "0xABCDEF0000000000000000000000000000000001")},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts:  []Receipt{receipt("0xabcdef0000000000000000000000000000000001", 1000000, "eth", "0xdeadbeef")},
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000},
		},
		{
			name:      "zkwithdraw is an L1 payment",
			invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync")},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts:  []Receipt{receipt(testWallet1, 1000000, "zkwithdraw", "0xdeadbeef")},
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "zkwithdraw:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000},
		},
		{
			name:      "mechanism casing and spacing are ignored",
			invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts:  []Receipt{receipt(testWallet1, 1000000, " zkSync-Era ", "0xdeadbeef")},
			payments: []payment{
				{nodeID: testNode1, amount: 1000000, receipt: "zkSync-Era:0xdeadbeef"},
			},
			distributed: map[NodeID]int64{testNode1: 1000000},
		},
		{
			name:      "unknown mechanism is rejected",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts:  []Receipt{receipt(testWallet1, 1000000, "polygon", "0xdeadbeef")},
			err:       `receipt has unknown payment mechanism "polygon"`,
		},
		{
			name:      "mechanism mismatch is not silently ignored",
			invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			// The node is paid out on L2 but the receipt is for an L1 payment.
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			err: "receipts cannot be reconciled with the paystubs:\n" +
				`receipt eth:0xdeadbeef transferred 1.000000 to wallet "0x0000000000000000000000000000000000000001", which matches no paystub`,
		},
		{
			name:      "duplicate receipt is rejected",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts: []Receipt{
				receipt(testWallet1, 1000000, "eth", "0xdeadbeef"),
				receipt(testWallet1, 1000000, "ETH", "0xfeedface"),
			},
			err: `duplicate receipt entry for {"0x0000000000000000000000000000000000000001" "eth"} found`,
		},
		{
			name:      "receipt without a paystub is rejected",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			receipts: []Receipt{
				receipt(testWallet1, 1000000, "eth", "0xdeadbeef"),
				receipt(testWallet2, 2000000, "eth", "0xfeedface"),
			},
			err: "receipts cannot be reconciled with the paystubs:\n" +
				`receipt eth:0xfeedface transferred 2.000000 to wallet "0x0000000000000000000000000000000000000002", which matches no paystub`,
		},
		{
			name: "receipt amount must match what is distributed",
			invoices: []Invoice{
				invoice(testNode1, testWallet1),
				invoice(testNode2, testWallet1),
			},
			ipaystubs: []IncompletePaystub{
				ipaystub(testNode1, 1000000),
				ipaystub(testNode2, 2500000),
			},
			// Only the payout of the first node was transferred.
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			err: "receipts cannot be reconciled with the paystubs:\n" +
				`receipt eth:0xdeadbeef transferred 1.000000 to wallet "0x0000000000000000000000000000000000000001", but 3.500000 was distributed to its nodes`,
		},
		{
			name:      "node missing from the invoices is rejected",
			invoices:  []Invoice{invoice(testNode1, testWallet1)},
			ipaystubs: []IncompletePaystub{ipaystub(testNode3, 1000000)},
			receipts:  []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			err:       `paystub for node "` + testNode3.String() + `" does not have a wallet`,
		},
		{
			name: "node with multiple wallets in the invoices is rejected",
			invoices: []Invoice{
				invoice(testNode1, testWallet1),
				invoice(testNode1, testWallet2),
			},
			err: `node has multiple wallets in invoices: "` + testNode1.String() + `"`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			paymentsOut := new(bytes.Buffer)
			paystubsOut := new(bytes.Buffer)

			err := Finalize(
				marshalCSV(t, tt.invoices),
				marshalCSV(t, tt.ipaystubs),
				marshalCSV(t, tt.receipts),
				paymentsOut, paystubsOut,
				FinalizeConfig{MaxUnpaidPercent: 5, AllowUnpaid: tt.allowUnpaid})
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				// Nothing may be written when the inputs cannot be reconciled.
				require.Empty(t, paymentsOut.String())
				require.Empty(t, paystubsOut.String())
				return
			}
			require.NoError(t, err)

			payments, err := ReadPayments(paymentsOut)
			require.NoError(t, err)

			require.Len(t, payments, len(tt.payments))
			for i, expected := range tt.payments {
				require.Equal(t, expected.nodeID, payments[i].NodeID)
				require.Equal(t, expected.amount, payments[i].Amount.Value())
				require.NotNil(t, payments[i].Receipt)
				require.Equal(t, expected.receipt, *payments[i].Receipt)
			}

			paystubs, err := ReadPaystubs(paystubsOut)
			require.NoError(t, err)

			// Every incomplete paystub must be completed, whether it was paid or not.
			require.Len(t, paystubs, len(tt.distributed))
			for _, paystub := range paystubs {
				distributed, ok := tt.distributed[paystub.NodeID]
				require.True(t, ok, "unexpected paystub for node %q", paystub.NodeID)
				require.Equal(t, distributed, paystub.Distributed.Value(), "distributed amount for node %q", paystub.NodeID)
			}
		})
	}
}

func TestNormalizeMechanism(t *testing.T) {
	for _, tt := range []struct {
		mechanism string
		expected  string
		err       string
	}{
		{mechanism: "eth", expected: "eth"},
		{mechanism: "ETH", expected: "eth"},
		{mechanism: "ethereum", expected: "eth"},
		{mechanism: "zkwithdraw", expected: "eth"},
		{mechanism: "zksync-era", expected: "zksync-era"},
		{mechanism: "zkSyncEra", expected: "zksync-era"},
		{mechanism: " zksync_era\t", expected: "zksync-era"},
		{mechanism: "zksync", err: `receipt has unknown payment mechanism "zksync"`},
		{mechanism: "polygon", err: `receipt has unknown payment mechanism "polygon"`},
		{mechanism: "", err: `receipt has unknown payment mechanism ""`},
	} {
		t.Run(tt.mechanism, func(t *testing.T) {
			feature, err := normalizeMechanism(tt.mechanism)
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, feature)
		})
	}
}

func TestFinalizeRandomNodeIDs(t *testing.T) {
	period, err := PeriodFromString("2020-01")
	require.NoError(t, err)

	nodeID := NodeID(testrand.NodeID())
	wallet := "0x" + string(testrand.RandAlphaNumeric(40))

	paymentsOut := new(bytes.Buffer)
	paystubsOut := new(bytes.Buffer)

	require.NoError(t, Finalize(
		marshalCSV(t, []Invoice{{Period: period, NodeID: nodeID, NodeWallet: wallet}}),
		marshalCSV(t, []IncompletePaystub{{Period: period, NodeID: nodeID, Codes: Codes{}, PossiblyDistributed: currency.NewMicroUnit(1234567)}}),
		marshalCSV(t, []Receipt{{Wallet: wallet, Amount: Amount(currency.NewMicroUnit(1234567)), TxHash: "0xdeadbeef", Mechanism: "eth"}}),
		paymentsOut, paystubsOut, FinalizeConfig{}))

	payments, err := ReadPayments(paymentsOut)
	require.NoError(t, err)
	require.Len(t, payments, 1)
	require.Equal(t, int64(1234567), payments[0].Amount.Value())
}

// marshalCSV renders the rows as the CSV input of Finalize.
func marshalCSV[T any](t *testing.T, rows []T) *bytes.Buffer {
	buf := new(bytes.Buffer)
	require.NoError(t, strictcsv.Write(buf, rows))
	return buf
}
