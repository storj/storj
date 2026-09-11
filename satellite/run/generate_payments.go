// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"io"
	"os"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/shared/modular"
)

// GeneratePaymentsConfig configures the compensation generate-payments subcommand.
type GeneratePaymentsConfig struct {
	Paystubs    string `help:"comma separated <satellite>:<path> list of the incomplete paystub CSVs, e.g. ap1:2026-07-ap1-incompletepaystubs.csv,us1:2026-07-us1-incompletepaystubs.csv" required:"true"`
	Receipts    string `help:"path to the receipts CSV of the executed payouts, covering every satellite listed in --paystubs" required:"true"`
	Invoices    string `help:"comma separated <satellite>:<path> list of the invoice CSVs, which map the nodes to their wallet and wallet features. Defaults to the --paystubs paths with their incompletepaystubs.csv suffix replaced by invoices.csv" default:""`
	Payments    string `help:"comma separated <satellite>:<path> list of the payment CSVs to write. Defaults to the --paystubs paths with their incompletepaystubs.csv suffix replaced by payments.csv" default:""`
	PaystubsOut string `help:"comma separated <satellite>:<path> list of the finalized paystub CSVs to write, which carry the distributed amount the payouts were attributed with. Record them together with the payments, using record-period (record-paystubs drops the distributed amount). Defaults to the --paystubs paths with their incompletepaystubs.csv suffix replaced by paystubs.csv" default:""`
	SummaryOut  string `help:"destination path of the summary table; empty means stdout" default:""`
	Continue    bool   `help:"write the payments even if the payouts cannot be attributed to the paystubs. A receipt that transferred less than its nodes are owed is not covered by it and stays an error, since crediting them the full amount writes the shortfall off for them" default:"false"`

	MaxUnpaidPercent int64 `help:"largest share of the payout that may have no receipt before the attribution fails, which is what catches a receipts file that does not describe this payout" default:"5"`
	AllowUnpaid      bool  `help:"write the payments even if a large share of the payout has no receipt" default:"false"`

	ZksyncBonusPercent int64  `help:"bonus paid on top of a payout executed on zkSync Era, in percent. A zkSync Era receipt transferring the payout plus this bonus is accepted as matching the paystubs; the payments still record the payout alone, and the bonus is reported separately. Zero expects no bonus" default:"0"`
	BonusTolerance     string `help:"largest difference between the transferred amount and the payout plus bonus that is still taken for a match, as a decimal amount. Covers how the payout tool rounded the bonus it computed" default:"0.001"`

	ZkSyncEraRetired bool `help:"Attribute the period as one that was paid entirely on L1, including the wallets announcing zkSync Era. Must match what the prepare run of the same period was given, and cannot be combined with a non-zero --zksync-bonus-percent" default:"false"`
}

// GeneratePayments is a subcommand that attributes the payouts of a receipts
// file back to the nodes of every satellite that took part in them, writing the
// payments and the finalized paystubs of each of them.
type GeneratePayments struct {
	log    *zap.Logger
	config *GeneratePaymentsConfig
	stop   *modular.StopTrigger
}

// NewGeneratePayments creates a new GeneratePayments command.
func NewGeneratePayments(log *zap.Logger, config *GeneratePaymentsConfig, stop *modular.StopTrigger) *GeneratePayments {
	return &GeneratePayments{log: log, config: config, stop: stop}
}

// incompletePaystubsSuffix is the tail of the paystub file names the invoice and
// payments paths are derived from, as written by generate-invoices and prepare.
const incompletePaystubsSuffix = "incompletepaystubs.csv"

// Run generates the payments of every satellite.
func (g *GeneratePayments) Run(ctx context.Context) (err error) {
	defer g.stop.Cancel()

	paystubs, err := parseSatelliteFiles("paystubs", g.config.Paystubs)
	if err != nil {
		return err
	}

	invoices, err := satelliteFilesFor("invoices", g.config.Invoices, paystubs, "invoices.csv")
	if err != nil {
		return err
	}

	payments, err := satelliteFilesFor("payments", g.config.Payments, paystubs, "payments.csv")
	if err != nil {
		return err
	}

	paystubsOut, err := satelliteFilesFor("paystubs-out", g.config.PaystubsOut, paystubs, "paystubs.csv")
	if err != nil {
		return err
	}

	// An empty tolerance is no tolerance: the transferred amount then has to
	// match the payout with the bonus exactly.
	bonusTolerance := currency.Zero
	if g.config.BonusTolerance != "" {
		bonusTolerance, err = currency.MicroUnitFromFloatString(g.config.BonusTolerance)
		if err != nil {
			return errs.New("invalid --bonus-tolerance %q: %v", g.config.BonusTolerance, err)
		}
		if bonusTolerance.Value() < 0 {
			return errs.New("--bonus-tolerance %q must not be negative", g.config.BonusTolerance)
		}
	}

	var closers []io.Closer
	defer func() {
		for _, closer := range closers {
			err = errs.Combine(err, closer.Close())
		}
	}()

	open := func(path string) (*os.File, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		closers = append(closers, file)
		return file, nil
	}

	receiptsIn, err := open(g.config.Receipts)
	if err != nil {
		return err
	}

	var outputs outputGroup
	defer outputs.Rollback()

	satellites := make([]compensation.SatellitePayout, 0, len(paystubs))
	for i, paystub := range paystubs {
		invoicesIn, err := open(invoices[i].Path)
		if err != nil {
			return err
		}
		ipaystubsIn, err := open(paystub.Path)
		if err != nil {
			return err
		}

		g.log.Info("reading satellite payout",
			zap.String("name", paystub.Name),
			zap.String("invoices", invoices[i].Path),
			zap.String("incompletepaystubs", paystub.Path),
			zap.String("payments", payments[i].Path),
			zap.String("paystubs", paystubsOut[i].Path))

		paymentsWriter, err := outputs.Open(payments[i].Path)
		if err != nil {
			return err
		}
		paystubsWriter, err := outputs.Open(paystubsOut[i].Path)
		if err != nil {
			return err
		}

		satellites = append(satellites, compensation.SatellitePayout{
			Name:               paystub.Name,
			Invoices:           invoicesIn,
			IncompletePaystubs: ipaystubsIn,
			Payments:           paymentsWriter,
			Paystubs:           paystubsWriter,
		})
	}

	// The summary is opened last, so that everything is written or nothing is: a
	// payments file left behind on its own would be recorded as if it described
	// the whole payout, and a paystub file left behind without its payments
	// would record the money as distributed without a receipt for it.
	summaryOut, err := outputs.Open(g.config.SummaryOut)
	if err != nil {
		return err
	}

	report, err := compensation.GeneratePayments(satellites, receiptsIn, compensation.GeneratePaymentsConfig{
		Continue:           g.config.Continue,
		MaxUnpaidPercent:   g.config.MaxUnpaidPercent,
		AllowUnpaid:        g.config.AllowUnpaid,
		ZksyncBonusPercent: g.config.ZksyncBonusPercent,
		BonusTolerance:     bonusTolerance,
		ZkSyncEraRetired:   g.config.ZkSyncEraRetired,
		Log:                g.log,
	})
	if err != nil {
		return err
	}

	if err := compensation.WritePaymentsSummary(summaryOut, report); err != nil {
		return err
	}

	if err := outputs.Commit(); err != nil {
		return err
	}

	g.log.Info("Generated payments", zap.Int("satellites", len(satellites)))
	return nil
}

// satelliteFile is one satellite's CSV path, as given in a
// <satellite>:<path> list.
type satelliteFile struct {
	Name string
	Path string
}

// parseSatelliteFiles parses a comma separated <satellite>:<path> list.
func parseSatelliteFiles(flag, value string) ([]satelliteFile, error) {
	var files []satelliteFile
	names := make(map[string]struct{})

	for _, entry := range strings.Split(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, path, ok := strings.Cut(entry, ":")
		name, path = strings.TrimSpace(name), strings.TrimSpace(path)
		if !ok || name == "" || path == "" {
			return nil, errs.New("--%s entry %q is not in the <satellite>:<path> form", flag, entry)
		}
		if _, ok := names[name]; ok {
			return nil, errs.New("--%s has more than one entry for satellite %q", flag, name)
		}
		names[name] = struct{}{}
		files = append(files, satelliteFile{Name: name, Path: path})
	}

	if len(files) == 0 {
		return nil, errs.New("--%s is empty", flag)
	}
	return files, nil
}

// satelliteFilesFor resolves the paths of a companion flag of --paystubs. When
// the flag is empty the paths are derived from the paystub paths by replacing
// their incompletepaystubs.csv suffix, otherwise the given list is reordered to
// match the paystubs and must name the very same satellites.
func satelliteFilesFor(flag, value string, paystubs []satelliteFile, suffix string) ([]satelliteFile, error) {
	if value == "" {
		files := make([]satelliteFile, 0, len(paystubs))
		for _, paystub := range paystubs {
			if !strings.HasSuffix(paystub.Path, incompletePaystubsSuffix) {
				return nil, errs.New("cannot derive the %s path of satellite %q from %q: it does not end in %q, so --%s has to be set explicitly",
					flag, paystub.Name, paystub.Path, incompletePaystubsSuffix, flag)
			}
			files = append(files, satelliteFile{
				Name: paystub.Name,
				Path: strings.TrimSuffix(paystub.Path, incompletePaystubsSuffix) + suffix,
			})
		}
		return files, nil
	}

	given, err := parseSatelliteFiles(flag, value)
	if err != nil {
		return nil, err
	}
	if len(given) != len(paystubs) {
		return nil, errs.New("--%s lists %d satellites but --paystubs lists %d", flag, len(given), len(paystubs))
	}

	byName := make(map[string]string, len(given))
	for _, file := range given {
		byName[file.Name] = file.Path
	}

	files := make([]satelliteFile, 0, len(paystubs))
	for _, paystub := range paystubs {
		path, ok := byName[paystub.Name]
		if !ok {
			return nil, errs.New("--%s has no entry for satellite %q", flag, paystub.Name)
		}
		files = append(files, satelliteFile{Name: paystub.Name, Path: path})
	}
	return files, nil
}
