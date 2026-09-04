// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/currency"
	"storj.io/storj/shared/strictcsv"
)

// SatellitePayout is the input and output of GeneratePayments for a single
// satellite. Invoices and IncompletePaystubs must come from the same
// generate-invoices → prepare run, since they are joined on node-id. Payments
// and Paystubs receive the payments and the finalized paystubs CSV generated for
// the satellite, which belong together: recording one without the other is what
// makes a node paid twice, or not at all.
type SatellitePayout struct {
	Name               string
	Invoices           io.Reader
	IncompletePaystubs io.Reader
	Payments           io.Writer
	Paystubs           io.Writer
}

// zkSyncEraFeature is the wallet feature, and the receipt mechanism it is
// matched with, that the payout bonus is paid on.
const zkSyncEraFeature = "zksync-era"

// GeneratePaymentsConfig configures how strict GeneratePayments is about
// receipts it cannot attribute back to the paystubs.
type GeneratePaymentsConfig struct {
	// Continue writes the payments even when the receipts cannot be reconciled
	// with the paystubs. The problems are reported either way, as warnings
	// instead of an error.
	//
	// It does not cover a receipt that transferred less than the nodes it pays
	// are owed. The nodes are credited what their paystubs say, so continuing
	// past a shortfall records money that never moved as distributed and loses
	// the difference for them; that stays an error.
	Continue bool

	// MaxUnpaidPercent is the largest share of the payout that may have no
	// receipt at all before the attribution fails, the same threshold Finalize
	// applies. A receipts file that does not describe the payout produces no
	// reconciliation problem — it simply matches nothing — so the amount left
	// without a receipt is what catches it.
	MaxUnpaidPercent int64
	// AllowUnpaid writes the outputs regardless of how much of the payout has no
	// receipt.
	AllowUnpaid bool

	// ZksyncBonusPercent is the bonus, in percent, paid on top of the payout of a
	// wallet when it is executed on zkSync Era. A zkSync Era receipt transferring
	// the payout plus that bonus is accepted as matching the paystubs, and the
	// bonus itself is reported. Zero expects no bonus at all.
	ZksyncBonusPercent int64

	// BonusTolerance is the largest difference between the transferred amount and
	// the payout plus bonus that is still taken for a match. The payout tool
	// computes the bonus on its own, so its result differs from the recomputed
	// one by rounding. It is only applied to the amount with the bonus; a receipt
	// transferring the plain payout still has to match it exactly.
	BonusTolerance currency.MicroUnit

	// ZkSyncEraRetired attributes the period as one that was paid entirely on
	// L1. It has to match what Prepare was run with for the same period, since
	// the mechanism of a paystub is re-derived from the wallet features of its
	// invoice; see ZkSyncEraRetired on PrepareConfig. No wallet is designated
	// for zkSync Era under it, so it cannot be combined with a bonus.
	ZkSyncEraRetired bool

	Log *zap.Logger
}

// PaymentsReport is what GeneratePayments attributed, per satellite and in
// total.
type PaymentsReport struct {
	Satellites []PaymentsSummary

	// Bonus is the total transferred on top of the payouts of the wallets paid on
	// zkSync Era, and BonusWallets is how many of them carried it. The bonus is
	// not part of any payment: it is paid by the transaction, not owed for a
	// node's usage, so only the payout itself is recorded for the nodes.
	Bonus        currency.MicroUnit
	BonusWallets int
	BonusPercent int64

	// Unpaid is the payout of the wallets no receipt was found for, UnpaidMax the
	// largest of them and UnpaidWallets how many there are. Unpaid is the same
	// money the unattributed column of the satellites adds up to, seen per wallet
	// instead of per satellite: the maximum tells a run where a few wallets were
	// missed apart from one where everything stayed below the payout threshold.
	Unpaid        currency.MicroUnit
	UnpaidMax     currency.MicroUnit
	UnpaidWallets int
}

// PaymentsSummary reports, for one satellite, how much the paystubs said could
// be distributed and how much of it was attributed to an executed payout.
type PaymentsSummary struct {
	Name                string
	Nodes               int
	Payments            int
	PossiblyDistributed currency.MicroUnit
	Paid                currency.MicroUnit
}

// Unattributed is the part of the possibly distributed amount that no receipt
// paid for: wallets that stayed below the payout threshold, and anything else
// the payout did not cover.
func (summary PaymentsSummary) Unattributed() currency.MicroUnit {
	return currency.NewMicroUnit(summary.PossiblyDistributed.Value() - summary.Paid.Value())
}

// payoutEntry is one node's share of the payout executed against a wallet.
type payoutEntry struct {
	satellite int
	// seq orders the entries of a satellite the way its paystubs were read, so
	// the generated payments follow the paystub file instead of the map
	// iteration order of the wallets.
	seq    int
	period Period
	nodeID NodeID
	amount currency.MicroUnit
}

// GeneratePayments attributes the payouts that were executed against the
// wallets back to the (satellite, node) pairs they were paid for, and writes the
// payments and the finalized paystubs of every satellite.
//
// A payout is executed per wallet and covers every satellite at once, so the
// only thing tying a transaction back to the individual nodes is the amount each
// node's paystub says would be distributed to it. The nodes of a wallet are
// therefore collected across all the satellites, and their possibly-distributed
// amounts have to add up to the amount the receipt says was transferred. Any
// wallet where they do not is reported, because the difference is money that
// either moved without being recorded for a node, or was recorded for a node
// without moving.
//
// The same wallet can be paid on both chains within the same run, so the
// mechanism is part of the key the nodes are grouped by, and each group is
// reconciled against the receipt of its own mechanism. The mechanism follows the
// wallet features of the individual node, which do not have to agree with those
// of the other nodes of the wallet: a wallet shared by several nodes can have
// zksync-era configured on some of them and be paid on L1 for the rest, on one
// and the same satellite.
//
// A payout executed on zkSync Era can carry a bonus on top of what the nodes are
// owed, which ZksyncBonusPercent describes. Such a receipt transfers more than
// the paystubs add up to and still matches; the bonus is reported separately and
// is not recorded for the nodes, whose payments keep the payout amount.
//
// ZkSyncEraRetired describes a period that was paid on L1 alone. There is no
// split to reconcile then: every node of a wallet is in the same group whatever
// its wallet features say, and a receipt still reported on zkSync Era is
// attributed as the L1 payment it was. A wallet covered by two receipts is
// rejected as a duplicate in that world, since nothing is left to tell which of
// its nodes each of them paid.
//
// Nodes with no wallet, or with the all-zero one, are ignored altogether: no
// payout can reach them, so they are in none of the reported amounts.
//
// The paystubs are completed the way Finalize completes them, and for the same
// reason: the distributed amount is what records that the money reached the
// node. Prepare carries TotalPaid - TotalDistributed over into the next period,
// so a paystub recorded with a zero distributed amount for a payout that was
// executed pays the node a second time. Every node a payment was written for
// therefore gets the attributed amount as its distributed amount, and every
// other node keeps a zero one, which leaves it owed and paid out in a later
// period.
//
// The two outputs of a satellite have to be recorded together, with
// record-period rather than record-paystubs: the latter deliberately drops the
// distributed amount, since it also accepts the incomplete paystubs, for which
// nothing proves that a payout was executed.
//
// A receipts file that does not belong to the payout is refused rather than
// attributed: it matches no wallet, so it raises no reconciliation problem and
// would quietly write every paystub with a zero distributed amount, which pays
// every node a second time. MaxUnpaidPercent bounds how much of the payout may
// have no receipt before that is assumed, as in Finalize.
func GeneratePayments(satellites []SatellitePayout, receiptsIn io.Reader, config GeneratePaymentsConfig) (PaymentsReport, error) {
	log := config.Log
	if log == nil {
		log = zap.NewNop()
	}

	// With zkSync Era retired no wallet is designated for it, so the bonus could
	// never be applied to one. Silently ignoring it would report a payout that
	// carried the bonus as not adding up, so the combination is refused instead.
	if config.ZkSyncEraRetired && config.ZksyncBonusPercent > 0 {
		return PaymentsReport{}, errs.New("a zkSync Era bonus cannot be expected of a payout executed entirely on L1")
	}

	report := PaymentsReport{BonusPercent: config.ZksyncBonusPercent}

	byWallet, err := readReceiptsByWallet(receiptsIn, config.ZkSyncEraRetired)
	if err != nil {
		return PaymentsReport{}, err
	}

	var problems []string

	// shortfalls counts the problems that are receipts transferring less than the
	// nodes they pay are owed. They are reported with the rest, but Continue does
	// not write them off, since doing so loses the difference for the nodes.
	var shortfalls int

	// groups collects the nodes of a (wallet, mechanism) pair across every
	// satellite, since one transaction pays all of them together.
	groups := make(map[featuredWallet][]payoutEntry)

	// Nodes without a wallet are counted only to be logged; nothing of them
	// reaches the summary.
	var ignored int64
	var ignoredNodes int
	summaries := make([]PaymentsSummary, 0, len(satellites))

	// completed holds the paystubs of every satellite, in the order they were
	// read, so a payoutEntry's seq indexes the paystub it came from. They all
	// start out with a zero distributed amount, and the ones a payout is
	// attributed to are filled in below.
	completed := make([][]Paystub, len(satellites))

	names := make(map[string]struct{}, len(satellites))
	for satIdx, satellite := range satellites {
		if _, ok := names[satellite.Name]; ok {
			return PaymentsReport{}, errs.New("duplicate satellite %q", satellite.Name)
		}
		names[satellite.Name] = struct{}{}

		// The invoices are read leniently, because a payout is attributed after
		// the fact and the files it has to be attributed from can come from an
		// older satellite version (e.g. one still writing the since-removed
		// node-address column). Only the node to wallet mapping is taken from
		// them, and an extra column cannot change that: the wallet and wallet
		// feature columns are still required to be present, and a wrong mapping
		// surfaces as a receipt that does not add up.
		nodeWallets, err := readNodeWallets(log, satellite.Invoices, lenientInvoices, config.ZkSyncEraRetired)
		if err != nil {
			return PaymentsReport{}, errs.New("satellite %q: %w", satellite.Name, err)
		}

		ipaystubs, err := ReadIncompletePaystubs(satellite.IncompletePaystubs)
		if err != nil {
			return PaymentsReport{}, errs.New("satellite %q: %w", satellite.Name, err)
		}

		completed[satIdx] = make([]Paystub, len(ipaystubs))

		var possiblyDistributed int64
		for seq, ipaystub := range ipaystubs {
			// Every node gets a paystub, whether a payout reached it or not, so
			// that the period is recorded for all of them exactly once.
			completed[satIdx][seq] = ipaystub.Complete(currency.Zero)

			// A node that is owed nothing is not part of any payout, so there is
			// nothing to attribute to it.
			if ipaystub.PossiblyDistributed.Value() == 0 {
				continue
			}

			wallet, ok := nodeWallets[ipaystub.NodeID]
			if !ok {
				return PaymentsReport{}, errs.New("satellite %q: paystub for node %q does not have a wallet", satellite.Name, ipaystub.NodeID)
			}

			// A node that never configured a wallet cannot be paid, so no payout
			// could ever have covered it. It is ignored entirely, down to its
			// amount being left out of the summary: it would otherwise show up
			// as a payout that was missed, and all such nodes share the same
			// address, which would collect them into one enormous wallet.
			if wallet.Address == "" || wallet.Address == zeroWallet {
				ignoredNodes++
				ignored += ipaystub.PossiblyDistributed.Value()
				continue
			}

			possiblyDistributed += ipaystub.PossiblyDistributed.Value()

			// The mechanism comes from the node's own wallet features, so the
			// nodes of a wallet do not have to agree on it, not even within one
			// satellite: each of them lands in the group of the chain it is
			// paid on.
			groups[wallet] = append(groups[wallet], payoutEntry{
				satellite: satIdx,
				seq:       seq,
				period:    ipaystub.Period,
				nodeID:    ipaystub.NodeID,
				amount:    ipaystub.PossiblyDistributed,
			})
		}

		summaries = append(summaries, PaymentsSummary{
			Name:                satellite.Name,
			Nodes:               len(ipaystubs),
			PossiblyDistributed: currency.NewMicroUnit(possiblyDistributed),
		})
	}

	// attributed holds the payments of each satellite, tagged with the position
	// of the paystub they came from.
	type attributedPayment struct {
		seq     int
		payment Payment
	}
	attributed := make([][]attributedPayment, len(satellites))

	var unpaid, unpaidMax int64

	for fwallet, entries := range groups {
		distributed := sumEntries(entries)

		receipt, ok := byWallet[fwallet]
		if !ok {
			// No payout was executed for the wallet, which is what happens when
			// it stayed below the payout threshold. The amount is simply not
			// attributed to anything; the paystubs of its nodes keep their zero
			// distributed amount, so it is still owed and paid out in a later
			// period, the same way Finalize leaves it. Only the total is
			// reported: a run has thousands of such wallets, and any of them
			// worth looking at is found through the maximum.
			report.UnpaidWallets++
			unpaid += distributed
			unpaidMax = max(unpaidMax, distributed)
			continue
		}

		transferred := currency.MicroUnit(receipt.Amount).Value()
		withBonus, hasBonus := bonusAmount(fwallet.Feature, distributed, config.ZksyncBonusPercent)

		switch {
		case transferred == distributed:
			// The payout was transferred as it is, without a bonus.
		case hasBonus && transferred >= distributed && abs(transferred-withBonus) <= config.BonusTolerance.Value():
			// The bonus is what the transaction paid on top of the payout, taken
			// as transferred instead of recomputed, so that the reported total is
			// the money that actually moved.
			//
			// The tolerance only covers how the payout tool rounded the bonus it
			// computed, which can only ever leave a surplus, so it is not allowed
			// to reach below the payout itself: a tolerance larger than the bonus
			// would otherwise match a transaction that moved less than the nodes
			// are owed, which would take it off the shortfall path below and
			// credit them the difference anyway.
			report.Bonus = currency.NewMicroUnit(report.Bonus.Value() + transferred - distributed)
			report.BonusWallets++
		default:
			problem := fmt.Sprintf("receipt %s:%s transferred %s to wallet %q, but %s was distributed to its nodes (%s)",
				fwallet.Feature, receipt.TxHash, currency.NewMicroUnit(transferred).FloatString(), fwallet.Address,
				currency.NewMicroUnit(distributed).FloatString(), describeEntries(satellites, entries))
			if hasBonus {
				problem += fmt.Sprintf("; with the %d%% bonus %s was expected",
					config.ZksyncBonusPercent, currency.NewMicroUnit(withBonus).FloatString())
			}
			if transferred < distributed {
				// The nodes are credited the amount their paystubs say, which is
				// more than the transaction moved, so the shortfall is written off
				// for them: the payment and the distributed amount both record
				// money that never arrived, and Prepare carries nothing over. There
				// is no reading of an under-transfer under which that is right, so
				// Continue does not cover it — unlike an over-transfer, where the
				// nodes still get exactly what they are owed and only the surplus
				// is unexplained.
				problem += "; the nodes would be credited more than the transaction moved"
				shortfalls++
			}
			problems = append(problems, problem)
		}

		// The mechanism of the receipt is kept as reported by the payout tool
		// (e.g. "zkwithdraw" instead of the "eth" feature it maps to), since the
		// recorded receipt is what ties the payment back to the transaction.
		receiptRef := strings.TrimSpace(receipt.Mechanism) + ":" + receipt.TxHash
		for _, entry := range entries {
			// The receipt proves the money moved, so the paystub records it as
			// distributed and the amount stops being owed. The bonus is not part
			// of it: it was paid by the transaction, not for the node's usage.
			completed[entry.satellite][entry.seq].Distributed = entry.amount

			attributed[entry.satellite] = append(attributed[entry.satellite], attributedPayment{
				seq: entry.seq,
				payment: Payment{
					Period:  entry.period,
					NodeID:  entry.nodeID,
					Amount:  entry.amount,
					Receipt: &receiptRef,
				},
			})
		}
	}

	for fwallet, receipt := range byWallet {
		if _, ok := groups[fwallet]; !ok {
			problems = append(problems, fmt.Sprintf("receipt %s:%s transferred %s to wallet %q, which matches no paystub",
				fwallet.Feature, receipt.TxHash, currency.MicroUnit(receipt.Amount).FloatString(), fwallet.Address))
		}
	}

	if len(problems) > 0 {
		slices.Sort(problems)
		if !config.Continue {
			return PaymentsReport{}, errs.New("the payouts cannot be attributed to the paystubs:\n%s", strings.Join(problems, "\n"))
		}
		if shortfalls > 0 {
			return PaymentsReport{}, errs.New("the payouts cannot be attributed to the paystubs, and %d of them transferred less than the nodes are owed, which --continue does not write off:\n%s",
				shortfalls, strings.Join(problems, "\n"))
		}
		for _, problem := range problems {
			log.Warn("attributing the payouts anyway", zap.String("problem", problem))
		}
	}

	if !config.AllowUnpaid && unpaid > 0 {
		// Nothing above catches a receipts file that does not describe this
		// payout: an empty, header-only or stale one matches no wallet, so it
		// raises no problem and every group simply falls through as unpaid. The
		// paystubs are then all written with a zero distributed amount while the
		// payout may well have been executed, and the next Prepare carries
		// TotalPaid - TotalDistributed over and pays every node a second time.
		// The amount left without a receipt is what distinguishes that from the
		// legitimate case of a few wallets below the payout threshold, so it is
		// held to the same threshold Finalize holds it to.
		var totalAmount int64
		for _, summary := range summaries {
			totalAmount += summary.PossiblyDistributed.Value()
		}
		if len(byWallet) == 0 {
			return PaymentsReport{}, errs.New("refusing to write payouts: the receipts file is empty but %d wallets (%s) are owed a payout (use AllowUnpaid to override)",
				report.UnpaidWallets, currency.NewMicroUnit(unpaid).FloatString())
		}
		if unpaid*100 > totalAmount*config.MaxUnpaidPercent {
			return PaymentsReport{}, errs.New("refusing to write payouts: %d wallets (%s of %s) have no receipt, which is more than %d%% of the payout (use AllowUnpaid to override)",
				report.UnpaidWallets, currency.NewMicroUnit(unpaid).FloatString(), currency.NewMicroUnit(totalAmount).FloatString(), config.MaxUnpaidPercent)
		}
	}

	for i, satellite := range satellites {
		entries := attributed[i]
		slices.SortFunc(entries, func(a, b attributedPayment) int { return cmp.Compare(a.seq, b.seq) })

		payments := make([]Payment, 0, len(entries))
		var paid int64
		for _, entry := range entries {
			paid += entry.payment.Amount.Value()
			payments = append(payments, entry.payment)
		}

		summaries[i].Payments = len(payments)
		summaries[i].Paid = currency.NewMicroUnit(paid)

		if err := WritePayments(satellite.Payments, payments); err != nil {
			return PaymentsReport{}, errs.New("satellite %q: %w", satellite.Name, err)
		}
		if err := strictcsv.Write(satellite.Paystubs, completed[i]); err != nil {
			return PaymentsReport{}, errs.New("satellite %q: %w", satellite.Name, err)
		}
	}

	report.Unpaid = currency.NewMicroUnit(unpaid)
	report.UnpaidMax = currency.NewMicroUnit(unpaidMax)

	log.Info("generated the payments",
		zap.Int("satellites", len(satellites)),
		zap.Int("receipts", len(byWallet)),
		zap.Int("wallets_without_receipt", report.UnpaidWallets),
		zap.String("amount_without_receipt", report.Unpaid.FloatString()),
		zap.String("ignored_amount_without_wallet", currency.NewMicroUnit(ignored).FloatString()),
		zap.Int("ignored_nodes_without_wallet", ignoredNodes),
		zap.String("bonus", report.Bonus.FloatString()),
		zap.Int("bonus_wallets", report.BonusWallets),
		zap.Int("problems", len(problems)))

	report.Satellites = summaries
	return report, nil
}

// bonusAmount returns the payout of a wallet with the bonus of its chain added,
// and whether a bonus is expected on it at all.
func bonusAmount(feature string, distributed, percent int64) (int64, bool) {
	if percent <= 0 || feature != zkSyncEraFeature || distributed == 0 {
		return distributed, false
	}
	// Rounded to the nearest micro unit; the payout tool rounds on its own, which
	// BonusTolerance covers.
	return distributed + (distributed*percent+50)/100, true
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// readReceiptsByWallet reads the receipts and keys them on the wallet and the
// feature their mechanism maps to, rejecting a wallet that has more than one
// receipt for the same mechanism: the amounts of the nodes could be attributed
// to either of them.
func readReceiptsByWallet(receiptsIn io.Reader, zkSyncEraRetired bool) (map[featuredWallet]Receipt, error) {
	receipts, err := ReadReceipts(receiptsIn)
	if err != nil {
		return nil, err
	}

	byWallet := make(map[featuredWallet]Receipt, len(receipts))
	for _, receipt := range receipts {
		feature, err := normalizeMechanism(receipt.Mechanism, zkSyncEraRetired)
		if err != nil {
			return nil, err
		}
		fwallet := featuredWallet{
			Address: strings.ToLower(strings.TrimSpace(receipt.Wallet)),
			Feature: feature,
		}
		if _, ok := byWallet[fwallet]; ok {
			return nil, errs.New("duplicate receipt entry for %q found", fwallet)
		}
		byWallet[fwallet] = receipt
	}
	return byWallet, nil
}

func sumEntries(entries []payoutEntry) int64 {
	var total int64
	for _, entry := range entries {
		total += entry.amount.Value()
	}
	return total
}

// describeEntries breaks the amount distributed to a wallet down per satellite,
// so a mismatch points at the satellite whose paystubs have to be looked at.
func describeEntries(satellites []SatellitePayout, entries []payoutEntry) string {
	perSatellite := make(map[int]int64, len(satellites))
	for _, entry := range entries {
		perSatellite[entry.satellite] += entry.amount.Value()
	}

	parts := make([]string, 0, len(perSatellite))
	for i, satellite := range satellites {
		amount, ok := perSatellite[i]
		if !ok {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", satellite.Name, currency.NewMicroUnit(amount).FloatString()))
	}
	return strings.Join(parts, ", ")
}

// WritePaymentsSummary writes the per satellite summary as a plain text table,
// followed by the bonus the zkSync Era payouts carried on top of it, if any.
func WritePaymentsSummary(w io.Writer, report PaymentsReport) (err error) {
	summaries := report.Satellites
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	row := func(name string, nodes, payments int, possiblyDistributed, paid, unattributed currency.MicroUnit) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(tw, "%s\t%d\t%d\t%s\t%s\t%s\n", name, nodes, payments,
			possiblyDistributed.FloatString(), paid.FloatString(), unattributed.FloatString())
	}

	if _, err := fmt.Fprintln(tw, "SATELLITE\tNODES\tPAYMENTS\tPOSSIBLY-DISTRIBUTED\tPAID\tUNATTRIBUTED"); err != nil {
		return errs.Wrap(err)
	}

	var nodes, payments int
	var possiblyDistributed, paid int64
	for _, summary := range summaries {
		nodes += summary.Nodes
		payments += summary.Payments
		possiblyDistributed += summary.PossiblyDistributed.Value()
		paid += summary.Paid.Value()
		row(summary.Name, summary.Nodes, summary.Payments, summary.PossiblyDistributed, summary.Paid, summary.Unattributed())
	}
	if len(summaries) > 1 {
		row("TOTAL", nodes, payments,
			currency.NewMicroUnit(possiblyDistributed),
			currency.NewMicroUnit(paid),
			currency.NewMicroUnit(possiblyDistributed-paid))
	}
	if err != nil {
		return errs.Wrap(err)
	}

	if _, err := fmt.Fprintln(tw); err != nil {
		return errs.Wrap(err)
	}

	// The bonus is money the transactions carried on top of the payouts, so it is
	// reported next to the table instead of in it: no node was paid it, and it is
	// not part of any column above.
	if report.BonusPercent > 0 {
		if _, err := fmt.Fprintf(tw, "zksync-era bonus paid (%d%%): %s, over %s\n",
			report.BonusPercent, report.Bonus.FloatString(), plural(report.BonusWallets, "wallet")); err != nil {
			return errs.Wrap(err)
		}
	}

	// The wallets no receipt was found for, which the unattributed column of the
	// table adds up to. The maximum separates a run that missed a few wallets
	// from one where every unpaid wallet stayed below the payout threshold.
	if _, err := fmt.Fprintf(tw, "unpaid amounts max: %s, sum: %s, over %s\n",
		report.UnpaidMax.FloatString(), report.Unpaid.FloatString(), plural(report.UnpaidWallets, "wallet")); err != nil {
		return errs.Wrap(err)
	}

	return errs.Wrap(tw.Flush())
}

// plural renders a count with its unit, in the singular when there is one of it.
func plural(count int, unit string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, unit)
	}
	return fmt.Sprintf("%d %ss", count, unit)
}
