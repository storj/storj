// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"io"
	"net"
	"strings"

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

	Log *zap.Logger
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
	for _, invoice := range invoices {
		toPay := invoice.Owed
		toDistribute := currency.NewMicroUnit(
			invoice.Owed.Value() + (invoice.TotalPaid.Value() - invoice.TotalDistributed.Value()),
		)
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
