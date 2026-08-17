// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

// mechanisms maps the mechanism spellings accepted in a receipts file to the
// wallet feature names returned by ChooseFeature. The payout tool reports a
// withdrawal of a zkSync Lite balance back to L1 as "zkwithdraw", which is an
// L1 payment as far as the wallet feature is concerned.
var mechanisms = map[string]string{
	"eth":        "eth",
	"ethereum":   "eth",
	"zkwithdraw": "eth",
	"zksync-era": "zksync-era",
	"zksyncera":  "zksync-era",
	"zksync_era": "zksync-era",
}

// FinalizeConfig configures how strict the finalization is about paystubs that
// were not covered by a receipt.
type FinalizeConfig struct {
	// MaxUnpaidPercent is the largest share of the total payout that may have no
	// receipt before finalization refuses to write the outputs. Nodes below the
	// payout threshold legitimately end up without a receipt, but a large share
	// means the receipts file does not describe the payout that was executed.
	MaxUnpaidPercent int64
	// AllowUnpaid writes the outputs regardless of how much of the payout has no
	// receipt.
	AllowUnpaid bool

	Log *zap.Logger
}

// Finalize consumes the invoices, incomplete paystubs and payment receipts,
// then produces the payments and the finalized paystubs.
func Finalize(invoicesIn, ipaystubsIn, receiptsIn io.Reader, paymentsOut, paystubsOut io.Writer, config FinalizeConfig) error {
	log := config.Log
	if log == nil {
		log = zap.NewNop()
	}

	nodeWallets, err := readNodeWallets(log, invoicesIn)
	if err != nil {
		return err
	}

	ipaystubs, err := ReadIncompletePaystubs(ipaystubsIn)
	if err != nil {
		return err
	}

	receipts, err := ReadReceipts(receiptsIn)
	if err != nil {
		return err
	}

	byWallet := make(map[featuredWallet]Receipt, len(receipts))
	for _, receipt := range receipts {
		feature, err := normalizeMechanism(receipt.Mechanism)
		if err != nil {
			return err
		}
		fwallet := featuredWallet{
			Address: strings.ToLower(strings.TrimSpace(receipt.Wallet)),
			Feature: feature,
		}
		if _, ok := byWallet[fwallet]; ok {
			return errs.New("duplicate receipt entry for %q found", fwallet)
		}
		byWallet[fwallet] = receipt
	}

	payments := make([]Payment, 0, len(ipaystubs))
	paystubs := make([]Paystub, 0, len(ipaystubs))

	// distributed sums up the amounts credited against each receipt, so they can
	// be reconciled with the amount the receipt says was transferred.
	distributed := make(map[featuredWallet]int64, len(byWallet))

	var unpaid int
	var unpaidAmount, totalAmount int64

	for _, ipaystub := range ipaystubs {
		totalAmount += ipaystub.PossiblyDistributed.Value()

		// Skip the paystub if it did not produce a payout. we must still complete
		// the paystub for import.
		if ipaystub.PossiblyDistributed.Value() == 0 {
			paystubs = append(paystubs, ipaystub.Complete(currency.Zero))
			continue
		}

		// grab the candidate receipts for the node id based on the wallet associated with it.
		wallet, ok := nodeWallets[ipaystub.NodeID]
		if !ok {
			return errs.New("paystub for node %q does not have a wallet", ipaystub.NodeID)
		}

		candidate, ok := byWallet[wallet]
		if !ok {
			// The payout was not executed for this wallet (it was below the payout
			// threshold and not mandatory), so the amount is carried over to the
			// next period by Prepare. Count and log it: the same thing happens when
			// a receipt exists but fails to match, in which case the node would be
			// paid a second time for money that already moved.
			unpaid++
			unpaidAmount += ipaystub.PossiblyDistributed.Value()
			log.Warn("no receipt for a node with a non-zero payout, carrying the amount over to the next period",
				zap.Stringer("node_id", ipaystub.NodeID),
				zap.String("wallet", wallet.Address),
				zap.String("mechanism", wallet.Feature),
				zap.String("amount", ipaystub.PossiblyDistributed.FloatString()))
			paystubs = append(paystubs, ipaystub.Complete(currency.Zero))
			continue
		}

		distributed[wallet] += ipaystub.PossiblyDistributed.Value()

		// The mechanism of the receipt is kept as reported by the payout tool
		// (e.g. "zkwithdraw" instead of the "eth" feature it maps to), since the
		// recorded receipt is what ties the payment back to the transaction.
		receipt := strings.TrimSpace(candidate.Mechanism) + ":" + candidate.TxHash
		payments = append(payments, Payment{
			Period:  ipaystub.Period,
			NodeID:  ipaystub.NodeID,
			Amount:  ipaystub.PossiblyDistributed,
			Receipt: &receipt,
		})
		paystubs = append(paystubs, ipaystub.Complete(ipaystub.PossiblyDistributed))
	}

	if err := reconcileReceipts(byWallet, distributed); err != nil {
		return err
	}

	if !config.AllowUnpaid && unpaidAmount > 0 {
		// An empty receipts file completes every paystub with distributed=0 while
		// the payout may well have been executed, which makes the next Prepare pay
		// all of it a second time. It never has a legitimate reason to accompany a
		// non-zero payout, so it is refused independently of the threshold.
		if len(byWallet) == 0 {
			return errs.New("refusing to write payouts: the receipts file is empty but %d nodes (%s) are owed a payout (use AllowUnpaid to override)",
				unpaid, currency.NewMicroUnit(unpaidAmount).FloatString())
		}
		if unpaidAmount*100 > totalAmount*config.MaxUnpaidPercent {
			return errs.New("refusing to write payouts: %d nodes (%s of %s) have no receipt, which is more than %d%% of the payout (use AllowUnpaid to override)",
				unpaid, currency.NewMicroUnit(unpaidAmount).FloatString(), currency.NewMicroUnit(totalAmount).FloatString(), config.MaxUnpaidPercent)
		}
	}

	log.Info("finalized the payouts",
		zap.Int("paystubs", len(paystubs)),
		zap.Int("payments", len(payments)),
		zap.Int("receipts", len(byWallet)),
		zap.Int("nodes_without_receipt", unpaid),
		zap.String("amount_without_receipt", currency.NewMicroUnit(unpaidAmount).FloatString()))

	if err := WritePayments(paymentsOut, payments); err != nil {
		return err
	}

	return strictcsv.Write(paystubsOut, paystubs)
}

// normalizeMechanism maps the mechanism of a receipt onto the wallet feature
// name used by the paystubs, rejecting mechanisms we do not know about. A
// mechanism that fails to match would silently complete the paystubs of the
// wallet with a zero distributed amount, which makes Prepare carry the amount
// over and pay the nodes a second time in the next period.
func normalizeMechanism(mechanism string) (string, error) {
	feature, ok := mechanisms[strings.ToLower(strings.TrimSpace(mechanism))]
	if !ok {
		return "", errs.New("receipt has unknown payment mechanism %q", mechanism)
	}
	return feature, nil
}

// reconcileReceipts checks that every receipt was applied to at least one
// paystub and that the amounts credited to the nodes of a wallet add up to the
// amount the receipt says was transferred. Anything else means the paystubs do
// not describe what happened on chain: money that moved but is recorded as not
// distributed is paid out again in the next period, and money recorded as
// distributed but never transferred is lost for the node.
func reconcileReceipts(byWallet map[featuredWallet]Receipt, distributed map[featuredWallet]int64) error {
	var problems []string
	for fwallet, receipt := range byWallet {
		credited, matched := distributed[fwallet]
		transferred := currency.MicroUnit(receipt.Amount)
		switch {
		case !matched:
			problems = append(problems, fmt.Sprintf("receipt %s:%s transferred %s to wallet %q, which matches no paystub",
				fwallet.Feature, receipt.TxHash, transferred.FloatString(), fwallet.Address))
		case credited != transferred.Value():
			problems = append(problems, fmt.Sprintf("receipt %s:%s transferred %s to wallet %q, but %s was distributed to its nodes",
				fwallet.Feature, receipt.TxHash, transferred.FloatString(), fwallet.Address, currency.NewMicroUnit(credited).FloatString()))
		}
	}
	if len(problems) == 0 {
		return nil
	}
	slices.Sort(problems)
	return errs.New("receipts cannot be reconciled with the paystubs:\n%s", strings.Join(problems, "\n"))
}

type featuredWallet struct {
	Address string
	Feature string
}

func readNodeWallets(log *zap.Logger, invoicesIn io.Reader) (map[NodeID]featuredWallet, error) {
	invoices, err := ReadInvoices(invoicesIn)
	if err != nil {
		return nil, err
	}

	nodeWallets := make(map[NodeID]featuredWallet, len(invoices))
	for _, invoice := range invoices {
		if _, ok := nodeWallets[invoice.NodeID]; ok {
			return nil, errs.New("node has multiple wallets in invoices: %q", invoice.NodeID)
		}

		nodeWallets[invoice.NodeID] = featuredWallet{
			Address: strings.ToLower(strings.TrimSpace(invoice.NodeWallet)),
			Feature: ChooseFeature(log, invoice.NodeID, invoice.NodeWalletFeatures),
		}
	}

	return nodeWallets, nil
}
