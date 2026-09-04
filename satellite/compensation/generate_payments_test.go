// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
)

var (
	testNode4   = NodeID(storj.NodeID{4})
	testWallet3 = "0x0000000000000000000000000000000000000003"
)

func TestGeneratePayments(t *testing.T) {
	period, err := PeriodFromString("2026-07")
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

	// satellite is one satellite's share of the inputs.
	type satellite struct {
		name      string
		invoices  []Invoice
		ipaystubs []IncompletePaystub
	}

	// payment describes the expected payment for a node.
	type payment struct {
		nodeID  NodeID
		amount  int64
		receipt string
	}

	// summary describes the expected summary of a satellite.
	type summary struct {
		nodes               int
		payments            int
		possiblyDistributed int64
		paid                int64
	}

	for _, tt := range []struct {
		name             string
		satellites       []satellite
		receipts         []Receipt
		cont             bool
		allowUnpaid      bool
		bonusPercent     int64
		bonusTolerance   int64
		zkSyncEraRetired bool

		payments      map[string][]payment
		summaries     map[string]summary
		bonus         int64
		bonusWallets  int
		unpaid        int64
		unpaidMax     int64
		unpaidWallets int
		err           string
	}{
		{
			name:       "nothing to attribute",
			satellites: []satellite{{name: "us1"}},
			payments:   map[string][]payment{"us1": nil},
			summaries:  map[string]summary{"us1": {}},
		},
		{
			name: "single satellite",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
		},
		{
			// The payout is executed once per wallet, so the amounts of the
			// nodes of every satellite have to add up to the single receipt.
			name: "wallet paid across two satellites",
			satellites: []satellite{
				{
					name:      "ap1",
					invoices:  []Invoice{invoice(testNode1, testWallet1)},
					ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
				},
				{
					name:      "us1",
					invoices:  []Invoice{invoice(testNode2, testWallet1)},
					ipaystubs: []IncompletePaystub{ipaystub(testNode2, 2500000)},
				},
			},
			receipts: []Receipt{receipt(testWallet1, 3500000, "eth", "0xdeadbeef")},
			payments: map[string][]payment{
				"ap1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"}},
				"us1": {{nodeID: testNode2, amount: 2500000, receipt: "eth:0xdeadbeef"}},
			},
			summaries: map[string]summary{
				"ap1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 2500000, paid: 2500000},
			},
		},
		{
			// The operator configured the same wallet for zkSync Era on one
			// satellite and left it on L1 on the other, so it is paid twice, on
			// two chains, and each transaction covers only its own nodes.
			name: "wallet paid on both chains",
			satellites: []satellite{
				{
					name:      "ap1",
					invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
					ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
				},
				{
					name:      "us1",
					invoices:  []Invoice{invoice(testNode2, testWallet1)},
					ipaystubs: []IncompletePaystub{ipaystub(testNode2, 2500000)},
				},
			},
			receipts: []Receipt{
				receipt(testWallet1, 1000000, "zksync-era", "0xaaaa"),
				receipt(testWallet1, 2500000, "eth", "0xbbbb"),
			},
			payments: map[string][]payment{
				"ap1": {{nodeID: testNode1, amount: 1000000, receipt: "zksync-era:0xaaaa"}},
				"us1": {{nodeID: testNode2, amount: 2500000, receipt: "eth:0xbbbb"}},
			},
			summaries: map[string]summary{
				"ap1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 2500000, paid: 2500000},
			},
		},
		{
			// A wallet below the payout threshold is not paid at all. Nothing is
			// attributed to its nodes, and their paystubs record a zero
			// distributed amount, so it is still owed in the next period.
			name: "wallets without a receipt",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1),
					invoice(testNode2, testWallet2),
					invoice(testNode3, testWallet3),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 30000),
					ipaystub(testNode3, 70000),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			// The dust is 9% of the payout here, above the threshold a real run
			// leaves it at; this case is about what is reported for it, not about
			// the threshold, which the cases below cover.
			allowUnpaid: true,
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 3, payments: 1, possiblyDistributed: 1100000, paid: 1000000},
			},
			// Reported per wallet, so the largest single one stands out from the
			// dust of the wallets below the payout threshold.
			unpaid:        100000,
			unpaidMax:     70000,
			unpaidWallets: 2,
		},
		{
			// Nothing can be transferred to a node without a wallet, so it is
			// ignored: it is in no wallet, and its amount is in none of the
			// reported totals.
			name: "nodes without a wallet are ignored",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1),
					invoice(testNode2, ""),
					invoice(testNode3, "0x0000000000000000000000000000000000000000"),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 4000000),
					ipaystub(testNode3, 2000000),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"}},
			},
			summaries: map[string]summary{
				// The 6.0 owed to the two nodes without a wallet is in neither
				// the possibly distributed nor the unattributed amount.
				"us1": {nodes: 3, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
		},
		{
			name: "node owed nothing is not paid",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1),
					invoice(testNode2, testWallet1),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 0),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 2, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
		},
		{
			name: "amount does not add up",
			satellites: []satellite{
				{
					name:      "ap1",
					invoices:  []Invoice{invoice(testNode1, testWallet1)},
					ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
				},
				{
					name:      "us1",
					invoices:  []Invoice{invoice(testNode2, testWallet1)},
					ipaystubs: []IncompletePaystub{ipaystub(testNode2, 2500000)},
				},
			},
			receipts: []Receipt{receipt(testWallet1, 3000000, "eth", "0xdeadbeef")},
			err: "the payouts cannot be attributed to the paystubs:\n" +
				`receipt eth:0xdeadbeef transferred 3.000000 to wallet "0x0000000000000000000000000000000000000001", ` +
				`but 3.500000 was distributed to its nodes (ap1: 1.000000, us1: 2.500000)` +
				`; the nodes would be credited more than the transaction moved`,
		},
		{
			// With --continue the amounts are attributed the only way they can
			// be, even though they do not add up to what was transferred. The
			// transaction moved more than the nodes are owed, so they are still
			// credited exactly their own amount and only the surplus is
			// unexplained.
			name: "amount does not add up but continues",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{receipt(testWallet1, 3000000, "eth", "0xdeadbeef")},
			cont:     true,
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xdeadbeef"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
		},
		{
			// The mirror case is not attributable at all: the nodes would be
			// credited the 1.000000 their paystubs say, of which only 0.400000
			// moved, so the paystub records the rest as distributed and it stops
			// being owed without ever having been paid. --continue does not
			// cover it.
			name: "a receipt transferring less than owed is not continued",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{receipt(testWallet1, 400000, "eth", "0xdeadbeef")},
			cont:     true,
			err: "the payouts cannot be attributed to the paystubs, and 1 of them transferred less than the nodes are owed, which --continue does not write off:\n" +
				`receipt eth:0xdeadbeef transferred 0.400000 to wallet "0x0000000000000000000000000000000000000001", ` +
				`but 1.000000 was distributed to its nodes (us1: 1.000000)` +
				`; the nodes would be credited more than the transaction moved`,
		},
		{
			name: "receipt matching no paystub",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{
				receipt(testWallet1, 1000000, "eth", "0xdeadbeef"),
				receipt(testWallet2, 500000, "eth", "0xfeedbeef"),
			},
			err: "the payouts cannot be attributed to the paystubs:\n" +
				`receipt eth:0xfeedbeef transferred 0.500000 to wallet "0x0000000000000000000000000000000000000002", which matches no paystub`,
		},
		{
			// A wallet shared by several nodes of one satellite is paid on both
			// chains when only some of the nodes announce zksync-era. The
			// mechanism is the node's own, so the nodes of the wallet split
			// between the two receipts.
			name: "one satellite pays a wallet on two chains",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1, "zksync-era"),
					invoice(testNode2, testWallet1),
					invoice(testNode3, testWallet1),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 2500000),
					ipaystub(testNode3, 500000),
				},
			}},
			receipts: []Receipt{
				receipt(testWallet1, 1000000, "zksync-era", "0xaaaa"),
				// The two nodes left on L1 are paid by a single transaction.
				receipt(testWallet1, 3000000, "eth", "0xbbbb"),
			},
			payments: map[string][]payment{
				"us1": {
					{nodeID: testNode1, amount: 1000000, receipt: "zksync-era:0xaaaa"},
					{nodeID: testNode2, amount: 2500000, receipt: "eth:0xbbbb"},
					{nodeID: testNode3, amount: 500000, receipt: "eth:0xbbbb"},
				},
			},
			summaries: map[string]summary{
				"us1": {nodes: 3, payments: 3, possiblyDistributed: 4000000, paid: 4000000},
			},
		},
		{
			// The split is still reconciled per chain: the zksync-era nodes of
			// the wallet may not be paid from its L1 transaction.
			name: "a chain of a split wallet does not add up",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1, "zksync-era"),
					invoice(testNode2, testWallet1),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 2500000),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 3500000, "eth", "0xbbbb")},
			err: "the payouts cannot be attributed to the paystubs:\n" +
				`receipt eth:0xbbbb transferred 3.500000 to wallet "0x0000000000000000000000000000000000000001", ` +
				`but 2.500000 was distributed to its nodes (us1: 2.500000)`,
		},
		{
			name: "node missing from the invoices is rejected",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode4, 1000000)},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xdeadbeef")},
			err:      `satellite "us1": paystub for node "` + testNode4.String() + `" does not have a wallet`,
		},
		{
			name: "duplicate satellite is rejected",
			satellites: []satellite{
				{name: "us1"},
				{name: "us1"},
			},
			err: `duplicate satellite "us1"`,
		},
		{
			// zkSync Era payouts carry a bonus on top of what the nodes are owed.
			// The receipt transfers more than the paystubs add up to and still
			// matches, but the node is only paid what it is owed.
			name: "zksync bonus is accepted and reported",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts:     []Receipt{receipt(testWallet1, 1100000, "zksync-era", "0xaaaa")},
			bonusPercent: 10,
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "zksync-era:0xaaaa"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
			bonus:        100000,
			bonusWallets: 1,
		},
		{
			// The payout tool computes the bonus on its own, so the transferred
			// amount differs from the recomputed one by rounding. The bonus is
			// reported as transferred, dust included.
			name: "zksync bonus within the tolerance",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts:       []Receipt{receipt(testWallet1, 1099993, "zksync-era", "0xaaaa")},
			bonusPercent:   10,
			bonusTolerance: 1000,
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "zksync-era:0xaaaa"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
			bonus:        99993,
			bonusWallets: 1,
		},
		{
			// A tolerance larger than the bonus would otherwise reach below the
			// payout itself and take an under-transfer for a rounded bonus: with
			// the 10% bonus 5.500000 was expected, and 4.600000 is within 1.000000
			// of it. The tolerance only covers how the bonus was rounded, which is
			// always a surplus, so this stays a shortfall.
			name: "a tolerance wider than the bonus is not a way under the payout",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 5000000)},
			}},
			receipts:       []Receipt{receipt(testWallet1, 4600000, "zksync-era", "0xaaaa")},
			bonusPercent:   10,
			bonusTolerance: 1000000,
			cont:           true,
			err: "the payouts cannot be attributed to the paystubs, and 1 of them transferred less than the nodes are owed, which --continue does not write off:\n" +
				`receipt zksync-era:0xaaaa transferred 4.600000 to wallet "0x0000000000000000000000000000000000000001", ` +
				`but 5.000000 was distributed to its nodes (us1: 5.000000); with the 10% bonus 5.500000 was expected` +
				`; the nodes would be credited more than the transaction moved`,
		},
		{
			name: "zksync amount beyond the tolerance",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts:       []Receipt{receipt(testWallet1, 1050000, "zksync-era", "0xaaaa")},
			bonusPercent:   10,
			bonusTolerance: 1000,
			err: "the payouts cannot be attributed to the paystubs:\n" +
				`receipt zksync-era:0xaaaa transferred 1.050000 to wallet "0x0000000000000000000000000000000000000001", ` +
				`but 1.000000 was distributed to its nodes (us1: 1.000000); with the 10% bonus 1.100000 was expected`,
		},
		{
			// A payout the bonus was not applied to still matches its plain
			// amount, and contributes no bonus.
			name: "zksync payout without the bonus",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts:     []Receipt{receipt(testWallet1, 1000000, "zksync-era", "0xaaaa")},
			bonusPercent: 10,
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "zksync-era:0xaaaa"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
		},
		{
			// The bonus is paid on zkSync Era only, so an L1 transaction still
			// has to transfer the payout exactly.
			name: "no bonus is expected on L1",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts:     []Receipt{receipt(testWallet1, 1100000, "eth", "0xaaaa")},
			bonusPercent: 10,
			err: "the payouts cannot be attributed to the paystubs:\n" +
				`receipt eth:0xaaaa transferred 1.100000 to wallet "0x0000000000000000000000000000000000000001", ` +
				`but 1.000000 was distributed to its nodes (us1: 1.000000)`,
		},
		{
			name: "duplicate receipt is rejected",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{
				receipt(testWallet1, 600000, "eth", "0xaaaa"),
				receipt(testWallet1, 400000, "zkwithdraw", "0xbbbb"),
			},
			err: `duplicate receipt entry for {"0x0000000000000000000000000000000000000001" "eth"} found`,
		},
		{
			// An empty receipts file matches no wallet, so it raises no
			// reconciliation problem at all: every group falls through as unpaid
			// and every paystub would be written with a zero distributed amount
			// for a payout that was executed, which pays the nodes again next
			// period. It never legitimately accompanies a non-zero payout.
			name: "an empty receipts file is refused",
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1)},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: nil,
			err: `refusing to write payouts: the receipts file is empty but 1 wallets (1.000000) ` +
				`are owed a payout (use AllowUnpaid to override)`,
		},
		{
			// A truncated receipts file raises no problem either: the receipts it
			// does carry reconcile, and a wallet missing from it is
			// indistinguishable from one below the payout threshold. Only the
			// share of the payout left without a receipt tells the two apart.
			name: "a receipts file missing most of the payout is refused",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1),
					invoice(testNode2, testWallet2),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 500000),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xaaaa")},
			err: `refusing to write payouts: 1 wallets (0.500000 of 1.500000) have no receipt, ` +
				`which is more than 5% of the payout (use AllowUnpaid to override)`,
		},
		{
			// The same run is written once the operator confirms the payout
			// really did miss those wallets.
			name: "AllowUnpaid writes the payout anyway",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1),
					invoice(testNode2, testWallet2),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 500000),
				},
			}},
			receipts:    []Receipt{receipt(testWallet1, 1000000, "eth", "0xaaaa")},
			allowUnpaid: true,
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xaaaa"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 2, payments: 1, possiblyDistributed: 1500000, paid: 1000000},
			},
			unpaid:        500000,
			unpaidMax:     500000,
			unpaidWallets: 1,
		},
		{
			// Dust below the threshold is what the check has to let through, or
			// no ordinary run would ever be attributed.
			name: "unpaid dust below the threshold is attributed",
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1),
					invoice(testNode2, testWallet2),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 10000),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "eth", "0xaaaa")},
			payments: map[string][]payment{
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "eth:0xaaaa"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 2, payments: 1, possiblyDistributed: 1010000, paid: 1000000},
			},
			unpaid:        10000,
			unpaidMax:     10000,
			unpaidWallets: 1,
		},
		{
			// With zkSync Era retired the nodes of a wallet are one group
			// whatever they announce, so the single L1 transaction pays all of
			// them. This is the "one satellite pays a wallet on two chains" case
			// after the switchover.
			name:             "retired zksync era pays the whole wallet on L1",
			zkSyncEraRetired: true,
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1, "zksync-era"),
					invoice(testNode2, testWallet1),
					invoice(testNode3, testWallet1, "zksync2"),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 2500000),
					ipaystub(testNode3, 500000),
				},
			}},
			receipts: []Receipt{receipt(testWallet1, 4000000, "eth", "0xbbbb")},
			payments: map[string][]payment{
				"us1": {
					{nodeID: testNode1, amount: 1000000, receipt: "eth:0xbbbb"},
					{nodeID: testNode2, amount: 2500000, receipt: "eth:0xbbbb"},
					{nodeID: testNode3, amount: 500000, receipt: "eth:0xbbbb"},
				},
			},
			summaries: map[string]summary{
				"us1": {nodes: 3, payments: 3, possiblyDistributed: 4000000, paid: 4000000},
			},
		},
		{
			// Reprocessing a period whose payout tool still reported the old
			// mechanism: the money moved to the wallet, so it is attributed as
			// the L1 payment it was rather than left owed and paid again.
			name:             "retired zksync era attributes a zksync-era receipt",
			zkSyncEraRetired: true,
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{receipt(testWallet1, 1000000, "zksync-era", "0xaaaa")},
			payments: map[string][]payment{
				// The receipt reference keeps the mechanism the payout tool
				// reported, so the payment still names the transaction.
				"us1": {{nodeID: testNode1, amount: 1000000, receipt: "zksync-era:0xaaaa"}},
			},
			summaries: map[string]summary{
				"us1": {nodes: 1, payments: 1, possiblyDistributed: 1000000, paid: 1000000},
			},
		},
		{
			// Nothing is left to say which of the wallet's nodes each of the two
			// transactions paid, so the split is refused instead of guessed at.
			name:             "retired zksync era rejects a wallet paid twice",
			zkSyncEraRetired: true,
			satellites: []satellite{{
				name: "us1",
				invoices: []Invoice{
					invoice(testNode1, testWallet1, "zksync-era"),
					invoice(testNode2, testWallet1),
				},
				ipaystubs: []IncompletePaystub{
					ipaystub(testNode1, 1000000),
					ipaystub(testNode2, 2500000),
				},
			}},
			receipts: []Receipt{
				receipt(testWallet1, 1000000, "zksync-era", "0xaaaa"),
				receipt(testWallet1, 2500000, "eth", "0xbbbb"),
			},
			err: `duplicate receipt entry for {"0x0000000000000000000000000000000000000001" "eth"} found`,
		},
		{
			// No wallet is designated for zkSync Era once it is retired, so a
			// bonus expected on it could never be applied to one.
			name:             "retired zksync era refuses a bonus",
			zkSyncEraRetired: true,
			bonusPercent:     10,
			satellites: []satellite{{
				name:      "us1",
				invoices:  []Invoice{invoice(testNode1, testWallet1, "zksync-era")},
				ipaystubs: []IncompletePaystub{ipaystub(testNode1, 1000000)},
			}},
			receipts: []Receipt{receipt(testWallet1, 1100000, "zksync-era", "0xaaaa")},
			err:      "a zkSync Era bonus cannot be expected of a payout executed entirely on L1",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			outs := make(map[string]*bytes.Buffer, len(tt.satellites))
			paystubOuts := make(map[string]*bytes.Buffer, len(tt.satellites))
			nodesOf := make(map[string][]NodeID, len(tt.satellites))
			satellites := make([]SatellitePayout, 0, len(tt.satellites))
			for _, sat := range tt.satellites {
				out, paystubOut := new(bytes.Buffer), new(bytes.Buffer)
				// Every satellite of a case has its own buffer, except for the
				// duplicate name case, where the collision does not matter.
				outs[sat.name] = out
				paystubOuts[sat.name] = paystubOut
				for _, ipaystub := range sat.ipaystubs {
					nodesOf[sat.name] = append(nodesOf[sat.name], ipaystub.NodeID)
				}
				satellites = append(satellites, SatellitePayout{
					Name:               sat.name,
					Invoices:           marshalCSV(t, sat.invoices),
					IncompletePaystubs: marshalCSV(t, sat.ipaystubs),
					Payments:           out,
					Paystubs:           paystubOut,
				})
			}

			report, err := GeneratePayments(satellites, marshalCSV(t, tt.receipts), GeneratePaymentsConfig{
				Continue:           tt.cont,
				MaxUnpaidPercent:   5,
				AllowUnpaid:        tt.allowUnpaid,
				ZksyncBonusPercent: tt.bonusPercent,
				BonusTolerance:     currency.NewMicroUnit(tt.bonusTolerance),
				ZkSyncEraRetired:   tt.zkSyncEraRetired,
			})
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				// Nothing may be written when the payouts cannot be attributed.
				for name, out := range outs {
					require.Empty(t, out.String(), "payments of satellite %q", name)
					require.Empty(t, paystubOuts[name].String(), "paystubs of satellite %q", name)
				}
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.bonus, report.Bonus.Value(), "bonus")
			require.Equal(t, tt.bonusWallets, report.BonusWallets, "bonus wallets")
			require.Equal(t, tt.bonusPercent, report.BonusPercent, "bonus percent")
			require.Equal(t, tt.unpaid, report.Unpaid.Value(), "unpaid")
			require.Equal(t, tt.unpaidMax, report.UnpaidMax.Value(), "unpaid max")
			require.Equal(t, tt.unpaidWallets, report.UnpaidWallets, "unpaid wallets")

			// What the table reports as unattributed is exactly the payout of the
			// wallets that were left unpaid.
			var unattributed int64
			for _, got := range report.Satellites {
				unattributed += got.Unattributed().Value()
			}
			require.Equal(t, report.Unpaid.Value(), unattributed,
				"unattributed must add up to the unpaid amount")

			require.Len(t, report.Satellites, len(tt.satellites))
			for _, got := range report.Satellites {
				expected, ok := tt.summaries[got.Name]
				require.True(t, ok, "unexpected summary for satellite %q", got.Name)
				require.Equal(t, expected.nodes, got.Nodes, "nodes of satellite %q", got.Name)
				require.Equal(t, expected.payments, got.Payments, "payments of satellite %q", got.Name)
				require.Equal(t, expected.possiblyDistributed, got.PossiblyDistributed.Value(), "possibly distributed of satellite %q", got.Name)
				require.Equal(t, expected.paid, got.Paid.Value(), "paid of satellite %q", got.Name)
				require.Equal(t, expected.possiblyDistributed-expected.paid,
					got.Unattributed().Value(), "unattributed of satellite %q", got.Name)
			}

			for name, out := range outs {
				payments, err := ReadPayments(out)
				require.NoError(t, err)

				expected := tt.payments[name]
				require.Len(t, payments, len(expected), "payments of satellite %q", name)
				for i, expected := range expected {
					require.Equal(t, expected.nodeID, payments[i].NodeID)
					require.Equal(t, expected.amount, payments[i].Amount.Value())
					require.Equal(t, period, payments[i].Period)
					require.NotNil(t, payments[i].Receipt)
					require.Equal(t, expected.receipt, *payments[i].Receipt)
					require.Nil(t, payments[i].Notes)
				}

				// A paystub is written for every node, in the order they were
				// read, and the amount it records as distributed is exactly the
				// payment written for it: anything less is paid a second time
				// by the next period, anything more is lost for the node.
				distributed := make(map[NodeID]int64, len(expected))
				for _, expected := range expected {
					distributed[expected.nodeID] = expected.amount
				}

				paystubs, err := ReadPaystubs(paystubOuts[name])
				require.NoError(t, err)
				require.Len(t, paystubs, len(nodesOf[name]), "paystubs of satellite %q", name)
				for i, nodeID := range nodesOf[name] {
					require.Equal(t, nodeID, paystubs[i].NodeID)
					require.Equal(t, period, paystubs[i].Period)
					require.Equal(t, distributed[nodeID], paystubs[i].Distributed.Value(),
						"distributed of node %q of satellite %q", nodeID, name)
				}
			}
		})
	}
}

// The generated payments must follow the order of the paystubs they came from,
// not the iteration order of the wallets they were attributed through.
func TestGeneratePaymentsOrder(t *testing.T) {
	period, err := PeriodFromString("2026-07")
	require.NoError(t, err)

	nodes := make([]NodeID, 0, 20)
	invoices := make([]Invoice, 0, 20)
	ipaystubs := make([]IncompletePaystub, 0, 20)
	receipts := make([]Receipt, 0, 20)
	for i := 1; i <= 20; i++ {
		nodeID := NodeID(storj.NodeID{byte(i)})
		wallet := "0x" + string(rune('a'+i%26)) + "000000000000000000000000000000000000000"
		nodes = append(nodes, nodeID)
		invoices = append(invoices, Invoice{Period: period, NodeID: nodeID, NodeWallet: wallet})
		ipaystubs = append(ipaystubs, IncompletePaystub{
			Period:              period,
			NodeID:              nodeID,
			Codes:               Codes{},
			PossiblyDistributed: currency.NewMicroUnit(int64(i) * 1000),
		})
		receipts = append(receipts, Receipt{
			Wallet:    wallet,
			Amount:    Amount(currency.NewMicroUnit(int64(i) * 1000)),
			TxHash:    "0xtx",
			Mechanism: "eth",
		})
	}

	out, paystubOut := new(bytes.Buffer), new(bytes.Buffer)
	_, err = GeneratePayments([]SatellitePayout{{
		Name:               "us1",
		Invoices:           marshalCSV(t, invoices),
		IncompletePaystubs: marshalCSV(t, ipaystubs),
		Payments:           out,
		Paystubs:           paystubOut,
	}}, marshalCSV(t, receipts), GeneratePaymentsConfig{})
	require.NoError(t, err)

	payments, err := ReadPayments(out)
	require.NoError(t, err)
	require.Len(t, payments, len(nodes))
	for i, nodeID := range nodes {
		require.Equal(t, nodeID, payments[i].NodeID)
	}

	paystubs, err := ReadPaystubs(paystubOut)
	require.NoError(t, err)
	require.Len(t, paystubs, len(nodes))
	for i, nodeID := range nodes {
		require.Equal(t, nodeID, paystubs[i].NodeID)
	}
}

// A payout is attributed after it was executed, so the invoices it has to be
// attributed from can come from an older satellite version carrying a column
// the Invoice struct no longer has.
func TestGeneratePaymentsInvoiceColumns(t *testing.T) {
	period, err := PeriodFromString("2026-07")
	require.NoError(t, err)

	const header = "period,node-id,node-created-at,node-disqualified,node-gracefulexit,node-wallet," +
		"node-wallet-features,node-address,node-last-ip,codes,usage-at-rest,usage-get,usage-put," +
		"usage-get-repair,usage-put-repair,usage-get-audit,comp-at-rest,comp-get,comp-put," +
		"comp-get-repair,comp-put-repair,comp-get-audit,surge-percent,owed,held,disposed," +
		"total-held,total-disposed,total-paid,total-distributed,voluntary-discount\n"
	row := "2026-07," + testNode1.String() + ",2020-01-01,,," + testWallet1 +
		",zksync-era,127.0.0.1:28967,1.2.3.4,,0,0,0,0,0,0,0,0,0,0,0,0,0,1500000,0,0,0,0,0,0,0\n"

	generate := func(t *testing.T, invoices string) (*bytes.Buffer, error) {
		out := new(bytes.Buffer)
		_, err := GeneratePayments([]SatellitePayout{{
			Name:     "ap1",
			Invoices: bytes.NewBufferString(invoices),
			IncompletePaystubs: marshalCSV(t, []IncompletePaystub{{
				Period:              period,
				NodeID:              testNode1,
				Codes:               Codes{},
				PossiblyDistributed: currency.NewMicroUnit(1500000),
			}}),
			Payments: out,
			Paystubs: new(bytes.Buffer),
		}}, marshalCSV(t, []Receipt{{
			Wallet:    testWallet1,
			Amount:    Amount(currency.NewMicroUnit(1500000)),
			TxHash:    "0xaaaa",
			Mechanism: "zksync-era",
		}}), GeneratePaymentsConfig{})
		return out, err
	}

	t.Run("an unmapped column is ignored", func(t *testing.T) {
		out, err := generate(t, header+row)
		require.NoError(t, err)

		payments, err := ReadPayments(out)
		require.NoError(t, err)
		require.Len(t, payments, 1)
		require.Equal(t, testNode1, payments[0].NodeID)
		// The wallet features of the invoice still decide which receipt the
		// node is attributed to.
		require.Equal(t, "zksync-era:0xaaaa", *payments[0].Receipt)
	})

	// Ignoring the columns that are not mapped does not make the columns the
	// attribution reads optional.
	t.Run("a missing wallet column is still rejected", func(t *testing.T) {
		_, err := generate(t,
			strings.Replace(header, "node-wallet,", "", 1)+
				strings.Replace(row, testWallet1+",", "", 1))
		require.Error(t, err)
		require.Contains(t, err.Error(), `field headers ["node-wallet"] missing from CSV`)
	})
}

func TestWritePaymentsSummary(t *testing.T) {
	report := PaymentsReport{
		Satellites: []PaymentsSummary{
			{
				Name:                "ap1",
				Nodes:               3,
				Payments:            2,
				PossiblyDistributed: currency.NewMicroUnit(3500000),
				Paid:                currency.NewMicroUnit(3000000),
			},
			{
				Name:                "us1",
				Nodes:               1,
				Payments:            1,
				PossiblyDistributed: currency.NewMicroUnit(1000000),
				Paid:                currency.NewMicroUnit(1000000),
			},
		},
	}

	const table = "SATELLITE  NODES  PAYMENTS  POSSIBLY-DISTRIBUTED  PAID      UNATTRIBUTED\n" +
		"ap1        3      2         3.500000              3.000000  0.500000\n" +
		"us1        1      1         1.000000              1.000000  0.000000\n" +
		"TOTAL      4      3         4.500000              4.000000  0.500000\n"

	// The unpaid line is what the unattributed column adds up to, per wallet. It
	// is always reported, so the run says how much was left unpaid even when
	// nothing was.
	t.Run("without a bonus", func(t *testing.T) {
		report := report
		report.Unpaid = currency.NewMicroUnit(500000)
		report.UnpaidMax = currency.NewMicroUnit(400000)
		report.UnpaidWallets = 2

		out := new(bytes.Buffer)
		require.NoError(t, WritePaymentsSummary(out, report))
		require.Equal(t, table+
			"\nunpaid amounts max: 0.400000, sum: 0.500000, over 2 wallets\n", out.String())
	})

	// The bonus is money the transactions carried on top of the payouts, so it
	// is reported next to the table and is in none of its columns.
	t.Run("with a bonus", func(t *testing.T) {
		report := report
		report.Bonus = currency.NewMicroUnit(300000)
		report.BonusWallets = 2
		report.BonusPercent = 10
		report.Unpaid = currency.NewMicroUnit(500000)
		report.UnpaidMax = currency.NewMicroUnit(500000)
		report.UnpaidWallets = 1

		out := new(bytes.Buffer)
		require.NoError(t, WritePaymentsSummary(out, report))
		require.Equal(t, table+
			"\nzksync-era bonus paid (10%): 0.300000, over 2 wallets\n"+
			"unpaid amounts max: 0.500000, sum: 0.500000, over 1 wallet\n", out.String())
	})
}
