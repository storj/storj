// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"
	"net"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/geoip"
	"storj.io/storj/shared/strictcsv"
)

// Amount is a currency amount that serializes to and from a float string in CSV.
type Amount currency.MicroUnit

// MarshalCSV serializes the amount as a float string.
func (amount Amount) MarshalCSV() (string, error) {
	return currency.MicroUnit(amount).FloatString(), nil
}

// UnmarshalCSV parses the amount from a float string.
func (amount *Amount) UnmarshalCSV(s string) error {
	m, err := currency.MicroUnitFromFloatString(s)
	if err != nil {
		return err
	}
	*amount = Amount(m)
	return nil
}

// Prepayout represents a payout to some sort of wallet address and if that eventual
// payout must be completed if every address only contains mandatory payouts.
type Prepayout struct {
	Address     string `csv:"address"`
	Amount      Amount `csv:"amount"`
	AddressKind string `csv:"address-kind"`
	Mandatory   bool   `csv:"mandatory"`
	Sanctioned  bool   `csv:"sanctioned"`
}

// PrepareConfig configures how invoices are turned into incomplete paystubs and prepayouts.
type PrepareConfig struct {
	ForceMandatory bool

	GeoIPDBs        []*geoip.MaxmindDB
	SkipOFAC        bool
	AllowUnscreened bool

	// Prepayment adds the prepayment (see prepayment) to what every node that
	// is staying in the network is both paid and paid out for the period.
	Prepayment bool

	Log *zap.Logger
}

// The prepayment models what a node would earn for the same period under a
// smaller expansion factor and at lower prices, and pays that on top of what
// the node earned under the rates the period was invoiced at.
var (
	// nominator and denominator are the smaller expansion factor the prepayment
	// is modelled at, relative to the one the period was invoiced under.
	nominator   = decimal.NewFromInt(30)
	denominator = decimal.NewFromInt(36)

	// prepaymentAtRestPercent is the 1.35 USD/TB/month the prepayment prices
	// data at rest at, as a percent of the 1.50 USD/TB/month the period is
	// invoiced at.
	prepaymentAtRestPercent = decimal.RequireFromString("90")

	// prepaymentEgressPercent is the share of the invoiced egress price the
	// prepayment pays for egress.
	prepaymentEgressPercent = decimal.RequireFromString("50")
)

// prepayment returns the amount to pay the node of the invoice on top of what
// the invoice says it is owed.
//
// It is calculated from the compensation of the invoice itself, so everything
// the invoice already accounts for - the length of the period, a partial
// period, and the price a node lowered for itself with self-signed price tags
// (see tag_rates.go) - is accounted for in the prepayment too. The compensation
// amounts are all pre-surge, and so is the prepayment: it is a fixed 30/36 at
// 1.35 model that is deliberately not inflated by a surge above 100. A surge
// below 100 is a different matter and is rejected, see below.
//
// Nodes that are on their way out of the network earn no prepayment: a
// disqualified node is not coming back, and a node that has gracefully exited
// or is exiting has announced it is leaving, so there is nothing to prepay it
// for. An offline node is not paid for the period at all - GenerateStatements
// zeroes its owed, held and disposed - so there is nothing to add a prepayment
// to either.
//
// No1099 and Sanctioned are a policy exclusion rather than a consequence of
// what Prepare already does. Neither code is emitted by GenerateStatements;
// both come from the external accountant on the invoice CSV, and Prepare does
// not otherwise act on them, so an invoice carrying one can still have a
// non-zero Owed that is paid out. The policy is that a node without a 1099, or
// one the accountant flagged as sanctioned, must not have a payout manufactured
// for it, so it earns no prepayment on top of whatever the invoice says.
//
// A node still in withholding earns none. The prepayment is a share of the
// gross compensation (GenerateStatements fills CompAtRest and the rest before
// held is subtracted from them), while Owed is that same compensation minus
// held, so at the first 75 percent withholding tier an at-rest-dominated node
// is owed 0.25*comp and would be prepaid 0.9*30/36*comp = 0.75*comp. It would
// be handed its whole escrow in cash while the paystub still records that
// escrow as held and graceful exit still owes it back (wallet_summary.go), so
// the withholding the escrow exists for would be gone.
//
// A surge below 100 percent is refused outright. Surge is the other reducer of
// the same shape as withholding: GenerateStatements scales only total by
// SurgePercent (statement.go), while CompAtRest and the rest are stored
// unscaled, so Owed is post-surge and the prepayment is not. At a surge of 50
// an at-rest-dominated node is owed 0.5*comp and would be prepaid
// 0.9*30/36*comp = 0.75*comp, so it is handed 2.5x what the period says it
// earned. Scaling the prepayment by the surge instead would contradict the
// fixed model above for a surge over 100, so the combination is an error rather
// than something to silently reinterpret.
func prepayment(invoice Invoice) (currency.MicroUnit, error) {
	if invoice.SurgePercent > 0 && invoice.SurgePercent < 100 {
		return currency.Zero, Error.New("prepayment is not supported at a surge below 100 percent (node %s invoiced at surge-percent %d): the prepayment is calculated from the pre-surge compensation while owed is post-surge, so it would exceed what the period earned", invoice.NodeID, invoice.SurgePercent)
	}

	if containsCode(invoice.Codes, Disqualified) ||
		containsCode(invoice.Codes, GracefulExit) ||
		containsCode(invoice.Codes, Offline) ||
		containsCode(invoice.Codes, No1099) ||
		containsCode(invoice.Codes, Sanctioned) ||
		containsCode(invoice.Codes, GracefulExiting) ||
		containsCode(invoice.Codes, InWithholding) {
		return currency.Zero, nil
	}

	atRest := PercentOf(invoice.CompAtRest.Decimal(), prepaymentAtRestPercent)
	egress := PercentOf(decimal.Sum(
		invoice.CompGet.Decimal(),
		invoice.CompGetRepair.Decimal(),
		invoice.CompGetAudit.Decimal(),
	), prepaymentEgressPercent)

	total := decimal.Sum(atRest, egress).
		Mul(nominator).
		Div(denominator)

	amount, err := currency.MicroUnitFromDecimal(total)
	if err != nil {
		return currency.Zero, Error.New("prepayment for node %s overflows: %v", invoice.NodeID, err)
	}
	return amount, nil
}

// Prepare reads invoices from invoicesIn and writes the resulting incomplete
// paystubs and prepayouts to the provided writers.
func Prepare(invoicesIn io.Reader, ipaystubsOut io.Writer, prepayoutsOut io.Writer, config PrepareConfig) error {
	log := config.Log
	if log == nil {
		log = zap.NewNop()
	}

	invoices, err := ReadInvoices(invoicesIn)
	if err != nil {
		return err
	}

	ipaystubs := make([]IncompletePaystub, 0, len(invoices))
	prepayouts := make([]Prepayout, 0, len(invoices))

	var unscreened int
	var prepaidNodes int
	var prepaidTotal int64
	for _, invoice := range invoices {
		toPay := invoice.Owed
		toDistribute := currency.NewMicroUnit(
			invoice.Owed.Value() + (invoice.TotalPaid.Value() - invoice.TotalDistributed.Value()),
		)

		var prepaid currency.MicroUnit
		if config.Prepayment {
			prepaid, err = prepayment(invoice)
			if err != nil {
				return err
			}
			// The prepayment is earned by the node for this period, so it is
			// both paid and paid out now and leaves no balance behind for the
			// next period to distribute. Adding it to what the period records
			// as paid, and not only to what it distributes, is what keeps the
			// TotalPaid >= TotalDistributed invariant (see db.go): were it only
			// distributed, the next period's Owed + (TotalPaid -
			// TotalDistributed) would be short by exactly the prepayment and
			// could turn negative.
			toPay = currency.NewMicroUnit(toPay.Value() + prepaid.Value())
			toDistribute = currency.NewMicroUnit(toDistribute.Value() + prepaid.Value())
		}

		codes := invoice.Codes

		sanction := false

		if !config.SkipOFAC {
			nodeIP := net.ParseIP(invoice.NodeLastIP)
			if nodeIP == nil {
				unscreened++
				log.Warn("skipping OFAC screening: no last IP recorded for node", zap.Stringer("node_id", invoice.NodeID))
			} else {
				var geoIPOK bool
				var geoIPErrs errs.Group
				for _, geoIPDB := range config.GeoIPDBs {
					loc, err := geoIPDB.LookupLocationByIP(nodeIP)
					if err != nil {
						geoIPErrs.Add(errs.New("failed to look up node %s location by IP %q: %v", invoice.NodeID, nodeIP, err))
						continue
					}
					geoIPOK = true
					if loc.Sanctioned {
						sanction = true
					}
				}
				if !geoIPOK {
					unscreened++
					log.Warn("OFAC screening failed for node", zap.Stringer("node_id", invoice.NodeID), zap.Error(geoIPErrs.Err()))
				}
			}
		}

		if sanction {
			codes = append(codes, Sanctioned)
			toPay = currency.NewMicroUnit(0)
			toDistribute = currency.NewMicroUnit(0)
			prepaid = currency.Zero
		}

		// Counted only once the screening above could still zero the payout,
		// so the summary reports what is actually being prepaid rather than
		// what was calculated.
		if prepaid.Value() != 0 {
			prepaidNodes++
			prepaidTotal += prepaid.Value()
		}

		ipaystubs = append(ipaystubs, IncompletePaystub{
			Period:              invoice.Period,
			NodeID:              invoice.NodeID,
			Codes:               codes,
			UsageAtRest:         invoice.UsageAtRest,
			UsageGet:            invoice.UsageGet,
			UsagePut:            invoice.UsagePut,
			UsageGetRepair:      invoice.UsageGetRepair,
			UsagePutRepair:      invoice.UsagePutRepair,
			UsageGetAudit:       invoice.UsageGetAudit,
			CompAtRest:          invoice.CompAtRest,
			CompGet:             invoice.CompGet,
			CompPut:             invoice.CompPut,
			CompGetRepair:       invoice.CompGetRepair,
			CompPutRepair:       invoice.CompPutRepair,
			CompGetAudit:        invoice.CompGetAudit,
			SurgePercent:        invoice.SurgePercent,
			Owed:                invoice.Owed,
			Held:                invoice.Held,
			Disposed:            invoice.Disposed,
			Paid:                toPay,
			PossiblyDistributed: toDistribute,
		})

		addressKind := ChooseFeature(log, invoice.NodeID, invoice.NodeWalletFeatures)

		prepayouts = append(prepayouts, Prepayout{
			Address:     invoice.NodeWallet,
			Amount:      Amount(toDistribute),
			AddressKind: addressKind,
			Mandatory:   config.ForceMandatory || isMandatory(invoice.Codes),
			Sanctioned:  sanction,
		})
	}

	if !config.SkipOFAC && !config.AllowUnscreened && unscreened > 0 {
		return errs.New("refusing to write payouts: %d nodes could not be OFAC-screened (use AllowUnscreened to override)", unscreened)
	}

	if config.Prepayment {
		log.Info("Prepayment applied",
			zap.Int("nodes", prepaidNodes),
			zap.String("total", currency.NewMicroUnit(prepaidTotal).FloatString()),
		)
	}

	if err := strictcsv.Write(ipaystubsOut, ipaystubs); err != nil {
		return err
	}

	if err := strictcsv.Write(prepayoutsOut, prepayouts); err != nil {
		return err
	}

	return nil
}

func containsCode(codes Codes, code Code) bool {
	for _, c := range codes {
		if code == c {
			return true
		}
	}
	return false
}

func isMandatory(codes Codes) bool {
	return containsCode(codes, Disqualified) ||
		containsCode(codes, GracefulExit)
}

// ChooseFeature picks the first payment method which is known by our system.
// It respects the preference of the operator from the L2 options. We prefer L2 over L1.
func ChooseFeature(log *zap.Logger, nodeID NodeID, features WalletFeatures) string {
	if log == nil {
		log = zap.NewNop()
	}
	for _, feature := range features {
		// handle if sno defined the list as one string
		for _, part := range strings.Split(feature, ",") {
			featureName := strings.Trim(part, `[]"“”`)
			featureName = strings.ReplaceAll(featureName, "-", "")
			switch strings.ToLower(featureName) {
			case "eth", "ethereum":
				// it's not an officially announced feature, but we don't need warning if sb. adds it
				continue
			case "zksyncera", "zksync2":
				return "zksync-era"
			case "zksync", "zkqync", "sksync", "zksynchistory", "zysync":
				return "eth"
			default:
				log.Warn("unknown wallet feature", zap.Stringer("node_id", nodeID), zap.String("feature", feature))
			}
		}
	}
	return "eth"
}

// IncompletePaystub contains the basic information about a payment that is to be made
// excluding information that is not determined until the payouts are executed.
type IncompletePaystub struct {
	Period              Period             `csv:"period"`
	NodeID              NodeID             `csv:"node-id"`
	Codes               Codes              `csv:"codes"`
	UsageAtRest         float64            `csv:"usage-at-rest"`
	UsageGet            int64              `csv:"usage-get"`
	UsagePut            int64              `csv:"usage-put"`
	UsageGetRepair      int64              `csv:"usage-get-repair"`
	UsagePutRepair      int64              `csv:"usage-put-repair"`
	UsageGetAudit       int64              `csv:"usage-get-audit"`
	CompAtRest          currency.MicroUnit `csv:"comp-at-rest"`
	CompGet             currency.MicroUnit `csv:"comp-get"`
	CompPut             currency.MicroUnit `csv:"comp-put"`
	CompGetRepair       currency.MicroUnit `csv:"comp-get-repair"`
	CompPutRepair       currency.MicroUnit `csv:"comp-put-repair"`
	CompGetAudit        currency.MicroUnit `csv:"comp-get-audit"`
	SurgePercent        int64              `csv:"surge-percent"`
	Owed                currency.MicroUnit `csv:"owed"`
	Held                currency.MicroUnit `csv:"held"`
	Disposed            currency.MicroUnit `csv:"disposed"`
	Paid                currency.MicroUnit `csv:"paid"`
	PossiblyDistributed currency.MicroUnit `csv:"possibly-distributed"`
}

// Complete converts an IncompletePaystub into a Paystub using the given distributed amount.
func (i IncompletePaystub) Complete(distributed currency.MicroUnit) Paystub {
	codes := make(Codes, 0, len(i.Codes))
	for _, code := range i.Codes {
		if code != Bonus { // until satellites support this code
			codes = append(codes, code)
		}
	}
	return Paystub{
		Period:         i.Period,
		NodeID:         i.NodeID,
		Codes:          codes,
		UsageAtRest:    i.UsageAtRest,
		UsageGet:       i.UsageGet,
		UsagePut:       i.UsagePut,
		UsageGetRepair: i.UsageGetRepair,
		UsagePutRepair: i.UsagePutRepair,
		UsageGetAudit:  i.UsageGetAudit,
		CompAtRest:     i.CompAtRest,
		CompGet:        i.CompGet,
		CompPut:        i.CompPut,
		CompGetRepair:  i.CompGetRepair,
		CompPutRepair:  i.CompPutRepair,
		CompGetAudit:   i.CompGetAudit,
		SurgePercent:   i.SurgePercent,
		Owed:           i.Owed,
		Held:           i.Held,
		Disposed:       i.Disposed,
		Paid:           i.Paid,
		Distributed:    distributed,
	}
}

// ReadIncompletePaystubs reads a collection of Paystubs in CSV form.
func ReadIncompletePaystubs(r io.Reader) ([]IncompletePaystub, error) {
	var paystubs []IncompletePaystub
	if err := strictcsv.Read(r, &paystubs); err != nil {
		return nil, err
	}
	return paystubs, nil
}
