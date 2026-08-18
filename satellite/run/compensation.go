// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/private/currency"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/compensation"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/shared/modular"
)

var mon = monkit.Package()

// GenerateInvoicesConfig configures the compensation-generate-invoices subcommand.
type GenerateInvoicesConfig struct {
	Period       string `help:"pay period to generate invoices for, a UTC date formatted like YYYY-MM" required:"true"`
	Output       string `help:"destination of report output" default:""`
	SurgePercent int64  `help:"surge percent for payments" default:"0"`
	RecentCutoff bool   `help:"if true, use the 24h before the period end (instead of the period start) as the cutoff for the offline and graceful-exiting checks. A node whose last successful contact is in that 24h window is treated as offline for the entire period and forfeits owed/held/disposed payments (including withheld-amount disposal), and only nodes still exiting at that cutoff are flagged GracefulExiting" default:"false"`
	Exclude      string `help:"Codes to be excluded from the final report, comma-separated" default:""`
	Cache        bool   `help:"preload per-node totals with one aggregate query instead of one query per node" default:"false"`
	StartDate    string `help:"optional partial-period start date (YYYY-MM-DD, inclusive). Must be set together with end-date and must fall inside --period. Overrides the period's month boundaries for usage aggregation and offline/DQ/GE/withholding checks. The paystub Period identifier still comes from --period, so only ONE partial run per --period may be recorded: record-period replaces the paystub on (period, node_id) and a second partial run would drop the first one's amounts from the lifetime totals." default:""`
	EndDate      string `help:"optional partial-period end date (YYYY-MM-DD, inclusive). See --start-date." default:""`
}

// RecordPeriodConfig configures the compensation-record-period subcommand.
type RecordPeriodConfig struct {
	PaystubsCSV string `help:"path to the paystubs CSV to record" required:"true"`
	PaymentsCSV string `help:"path to the payments CSV to record" required:"true"`
}

// RecordOneOffPaymentsConfig configures the compensation-record-one-off-payments subcommand.
type RecordOneOffPaymentsConfig struct {
	PaymentsCSV string `help:"path to the payments CSV to record" required:"true"`
}

// FinalizeConfig configures the compensation-finalize subcommand.
type FinalizeConfig struct {
	InvoicesCSV           string `help:"path to the invoices CSV" required:"true"`
	IncompletePaystubsCSV string `help:"path to the incomplete paystubs CSV" required:"true"`
	ReceiptsCSV           string `help:"path to the receipts CSV" required:"true"`
	PaymentsOut           string `help:"destination path for the payments CSV" required:"true"`
	PaystubsOut           string `help:"destination path for the paystubs CSV" required:"true"`
	MaxUnpaidPercent      int64  `help:"largest share of the payout that may have no receipt before the finalization fails" default:"5"`
	AllowUnpaid           bool   `help:"Write payouts even if a large share of the payout has no receipt"`
}

// GenerateInvoices is a tool subcommand that generates storage node invoices for
// a pay period. It mirrors the `compensation generate-invoices` command of the
// non-modular satellite.
type GenerateInvoices struct {
	log    *zap.Logger
	db     satellite.DB
	comp   compensation.Config
	config *GenerateInvoicesConfig
	stop   *modular.StopTrigger
}

// NewGenerateInvoices creates a new GenerateInvoices command.
func NewGenerateInvoices(log *zap.Logger, db satellite.DB, comp compensation.Config, config *GenerateInvoicesConfig, stop *modular.StopTrigger) *GenerateInvoices {
	return &GenerateInvoices{
		log:    log,
		db:     db,
		comp:   comp,
		config: config,
		stop:   stop,
	}
}

// Run generates the invoices and writes them to the configured output.
func (g *GenerateInvoices) Run(ctx context.Context) (err error) {
	defer g.stop.Cancel()

	period, err := compensation.PeriodFromString(g.config.Period)
	if err != nil {
		return err
	}

	if err := g.db.CheckVersion(ctx); err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	var totalDiscount currency.MicroUnit
	var discountedNodes int
	if err := runWithOutput(g.config.Output, func(out io.Writer) error {
		totalDiscount, discountedNodes, err = g.generateInvoicesCSV(ctx, period, out)
		return err
	}); err != nil {
		return err
	}

	if g.config.Output != "" {
		g.log.Info("Generated invoices")
	}
	// The sum is the gross pre-surge, pre-withholding discount (the raw
	// rate delta reported in Statement.VoluntaryDiscount) and is not the
	// actual reduction in Owed once surge and withholding are applied.
	g.log.Info("Total voluntary discount applied (pre-surge, pre-withholding)",
		zap.String("amount", totalDiscount.FloatString()),
		zap.Int("nodes", discountedNodes),
	)
	return nil
}

func (g *GenerateInvoices) generateInvoicesCSV(ctx context.Context, period compensation.Period, out io.Writer) (totalDiscount currency.MicroUnit, discountedNodes int, err error) {
	periodInfo := compensation.PeriodInfo{
		Period:           period,
		Rates:            &g.comp.Rates,
		SurgePercent:     g.config.SurgePercent,
		DisposePercent:   g.comp.DisposePercent,
		WithheldPercents: g.comp.WithheldPercents,
		Log:              g.log,
	}

	rangeStart, rangeEndExclusive, partial, err := parsePartialRange(period, g.config.StartDate, g.config.EndDate)
	if err != nil {
		return currency.Zero, 0, err
	}

	// endExclusive is the end of the range the statements are generated for: the
	// end of the month, or the end of the partial range when one is given.
	endExclusive := period.EndDateExclusive()
	if partial {
		endExclusive = rangeEndExclusive
		periodInfo.StartDateOverride = &rangeStart
		periodInfo.EndDateExclusiveOverride = &rangeEndExclusive
		g.log.Info("Generating invoices for partial period",
			zap.String("period", period.String()),
			zap.Time("start", rangeStart),
			zap.Time("end_exclusive", rangeEndExclusive),
		)
	}

	if g.config.RecentCutoff {
		// Derived from the effective end of the range, otherwise a partial run
		// would compare the nodes' last contact against the end of the whole
		// month and flag almost every node as Offline.
		periodInfo.Cutoff = endExclusive.Add(-24 * time.Hour)
	}

	excludeCodes := make(map[compensation.Code]struct{})
	for _, s := range strings.Split(g.config.Exclude, ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		code, err := compensation.CodeFromString(s)
		if err != nil {
			return currency.Zero, 0, errs.New("invalid exclude code %q: %+v", s, err)
		}
		excludeCodes[code] = struct{}{}
	}

	var periodUsage []accounting.StorageNodePeriodUsage
	if partial {
		periodUsage, err = g.db.StoragenodeAccounting().QueryStorageNodePeriodUsageRange(ctx, rangeStart, rangeEndExclusive)
	} else {
		periodUsage, err = g.db.StoragenodeAccounting().QueryStorageNodePeriodUsage(ctx, period)
	}
	if err != nil {
		return currency.Zero, 0, err
	}

	periodUsageByNode := make(map[storj.NodeID]accounting.StorageNodePeriodUsage, len(periodUsage))
	for _, usage := range periodUsage {
		periodUsageByNode[usage.NodeID] = usage
	}

	loadCounter := mon.Counter("loading")
	var allNodes []*overlay.NodeDossier
	err = g.db.OverlayCache().IterateAllNodeDossiers(ctx,
		func(ctx context.Context, node *overlay.NodeDossier) error {
			loadCounter.Inc(1)
			allNodes = append(allNodes, node)
			return nil
		})
	if err != nil {
		return currency.Zero, 0, err
	}

	var totalsCache map[storj.NodeID]compensation.TotalAmounts
	if g.config.Cache {
		totalsCache, err = g.db.Compensation().QueryAllTotalAmounts(ctx)
		if err != nil {
			return currency.Zero, 0, errs.New("failed to preload totals cache: %+v", err)
		}
		g.log.Info("Loaded totals cache", zap.Int("nodes", len(totalsCache)))
	}

	invoices := make([]compensation.Invoice, 0, len(allNodes))
	progressCounter := mon.Counter("progress")
	for _, node := range allNodes {
		progressCounter.Inc(1)
		totalAmounts, cached := totalsCache[node.Id]
		if !cached {
			totalAmounts, err = g.db.Compensation().QueryTotalAmounts(ctx, node.Id)
			if err != nil {
				return currency.Zero, 0, err
			}
			if totalsCache != nil {
				totalsCache[node.Id] = totalAmounts
			}
		}

		var gracefulExit *time.Time
		if node.ExitStatus.ExitSuccess {
			gracefulExit = node.ExitStatus.ExitFinishedAt
		}
		nodeAddress, _, err := net.SplitHostPort(node.Address.Address)
		if err != nil {
			return currency.Zero, 0, errs.New("unable to split node %q address %q", node.Id, node.Address.Address)
		}
		var nodeLastIP string
		if node.LastIPPort != "" {
			nodeLastIP, _, err = net.SplitHostPort(node.LastIPPort)
			if err != nil {
				return currency.Zero, 0, errs.New("unable to split node %q last ip:port %q", node.Id, node.LastIPPort)
			}
		}

		// the zero value of period usage is acceptable for if the node does not have
		// any usage for the period.
		usage := periodUsageByNode[node.Id]
		nodeInfo := compensation.NodeInfo{
			ID:                 node.Id,
			CreatedAt:          node.CreatedAt,
			LastContactSuccess: node.Reputation.LastContactSuccess,
			Disqualified:       node.Disqualified,
			GracefulExit:       gracefulExit,
			ExitInitiated:      node.ExitStatus.ExitInitiatedAt,
			UsageAtRest:        usage.AtRestTotal,
			UsageGet:           usage.GetTotal,
			UsagePut:           usage.PutTotal,
			UsageGetRepair:     usage.GetRepairTotal,
			UsagePutRepair:     usage.PutRepairTotal,
			UsageGetAudit:      usage.GetAuditTotal,
			TotalHeld:          totalAmounts.TotalHeld,
			TotalDisposed:      totalAmounts.TotalDisposed,
			TotalPaid:          totalAmounts.TotalPaid,
			TotalDistributed:   totalAmounts.TotalDistributed,
			Tags:               node.Tags,
		}

		invoice := compensation.Invoice{
			Period:             period,
			NodeID:             compensation.NodeID(node.Id),
			NodeWallet:         node.Operator.Wallet,
			NodeWalletFeatures: node.Operator.WalletFeatures,
			NodeAddress:        nodeAddress,
			NodeLastIP:         nodeLastIP,
		}

		if err := invoice.MergeNodeInfo(nodeInfo); err != nil {
			return currency.Zero, 0, err
		}
		invoices = append(invoices, invoice)
		periodInfo.Nodes = append(periodInfo.Nodes, nodeInfo)
	}

	statements, err := compensation.GenerateStatements(periodInfo)
	if err != nil {
		return currency.Zero, 0, err
	}

	isExcluded := func(inv compensation.Invoice) bool {
		for _, c := range inv.Codes {
			if _, ok := excludeCodes[c]; ok {
				return true
			}
		}
		return false
	}

	sum := int64(0)
	for i := range statements {
		if err := invoices[i].MergeStatement(statements[i]); err != nil {
			return currency.Zero, 0, err
		}
		if isExcluded(invoices[i]) {
			continue
		}
		if statements[i].VoluntaryDiscount.Value() > 0 {
			discountedNodes++
			sum += statements[i].VoluntaryDiscount.Value()
		}
	}

	if len(excludeCodes) > 0 {
		filtered := invoices[:0]
		for _, inv := range invoices {
			if !isExcluded(inv) {
				filtered = append(filtered, inv)
			}
		}
		invoices = filtered
	}

	if err := compensation.WriteInvoices(out, invoices); err != nil {
		return currency.Zero, 0, err
	}
	return currency.NewMicroUnit(sum), discountedNodes, nil
}

// RecordPeriod is a tool subcommand that records storage node paystubs and
// payments for a pay period. It mirrors the `compensation record-period`
// command of the non-modular satellite.
type RecordPeriod struct {
	log    *zap.Logger
	db     satellite.DB
	config *RecordPeriodConfig
	stop   *modular.StopTrigger
}

// NewRecordPeriod creates a new RecordPeriod command.
func NewRecordPeriod(log *zap.Logger, db satellite.DB, config *RecordPeriodConfig, stop *modular.StopTrigger) *RecordPeriod {
	return &RecordPeriod{
		log:    log,
		db:     db,
		config: config,
		stop:   stop,
	}
}

// Run records the paystubs and payments for a pay period.
func (r *RecordPeriod) Run(ctx context.Context) (err error) {
	defer r.stop.Cancel()

	paystubs, err := compensation.LoadPaystubs(r.config.PaystubsCSV)
	if err != nil {
		return err
	}

	payments, err := compensation.LoadPayments(r.config.PaymentsCSV)
	if err != nil {
		return err
	}

	if err := r.db.CheckVersion(ctx); err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	if err := r.db.Compensation().RecordPeriod(ctx, paystubs, payments); err != nil {
		return err
	}

	r.log.Info("Recorded pay period",
		zap.Int("paystubs", len(paystubs)),
		zap.Int("payments", len(payments)),
	)
	return nil
}

// RecordOneOffPayments is a tool subcommand that records one-off storage node
// payments outside of a pay period. It mirrors the
// `compensation record-one-off-payments` command of the non-modular satellite.
type RecordOneOffPayments struct {
	log    *zap.Logger
	db     satellite.DB
	config *RecordOneOffPaymentsConfig
	stop   *modular.StopTrigger
}

// NewRecordOneOffPayments creates a new RecordOneOffPayments command.
func NewRecordOneOffPayments(log *zap.Logger, db satellite.DB, config *RecordOneOffPaymentsConfig, stop *modular.StopTrigger) *RecordOneOffPayments {
	return &RecordOneOffPayments{
		log:    log,
		db:     db,
		config: config,
		stop:   stop,
	}
}

// Run records the one-off payments.
func (r *RecordOneOffPayments) Run(ctx context.Context) (err error) {
	defer r.stop.Cancel()

	payments, err := compensation.LoadPayments(r.config.PaymentsCSV)
	if err != nil {
		return err
	}

	if err := r.db.CheckVersion(ctx); err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	if err := r.db.Compensation().RecordPayments(ctx, payments); err != nil {
		return err
	}

	r.log.Info("Recorded one-off payments", zap.Int("payments", len(payments)))
	return nil
}

// Finalize is a tool subcommand that consumes invoices, incomplete paystubs
// and payment receipts to produce the final payments and paystubs CSVs.
type Finalize struct {
	log    *zap.Logger
	config *FinalizeConfig
	stop   *modular.StopTrigger
}

// NewFinalize creates a new Finalize command.
func NewFinalize(log *zap.Logger, config *FinalizeConfig, stop *modular.StopTrigger) *Finalize {
	return &Finalize{
		log:    log,
		config: config,
		stop:   stop,
	}
}

// Run executes the finalize step.
func (f *Finalize) Run(ctx context.Context) (err error) {
	defer f.stop.Cancel()

	invoicesIn, err := os.Open(f.config.InvoicesCSV)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, invoicesIn.Close()) }()

	ipaystubsIn, err := os.Open(f.config.IncompletePaystubsCSV)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, ipaystubsIn.Close()) }()

	receiptsIn, err := os.Open(f.config.ReceiptsCSV)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, receiptsIn.Close()) }()

	err = runWithOutputs([]string{f.config.PaymentsOut, f.config.PaystubsOut}, func(outs []io.Writer) error {
		return compensation.Finalize(invoicesIn, ipaystubsIn, receiptsIn, outs[0], outs[1], compensation.FinalizeConfig{
			MaxUnpaidPercent: f.config.MaxUnpaidPercent,
			AllowUnpaid:      f.config.AllowUnpaid,
			Log:              f.log,
		})
	})
	if err != nil {
		return err
	}

	f.log.Info("Finalized payments and paystubs",
		zap.String("payments", f.config.PaymentsOut),
		zap.String("paystubs", f.config.PaystubsOut),
	)
	return nil
}

// parsePartialRange parses the optional start/end YYYY-MM-DD flags. The end
// date is inclusive; the returned endExclusive is one day past it. Returns
// partial=false when both are empty (whole-month mode).
//
// The range must be contained in period, because the paystub written later by
// record-period is keyed on (period, node_id) and replaced on conflict: a
// second partial run for the same --period silently overwrites the paystub of
// the first one, dropping its held/owed/paid amounts from the lifetime totals
// that later withholding and disposal calculations read back.
func parsePartialRange(period compensation.Period, startStr, endStr string) (start, endExclusive time.Time, partial bool, err error) {
	if startStr == "" && endStr == "" {
		return time.Time{}, time.Time{}, false, nil
	}
	if startStr == "" || endStr == "" {
		return time.Time{}, time.Time{}, false, errs.New("--start-date and --end-date must be set together")
	}
	start, err = time.ParseInLocation("2006-01-02", startStr, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, false, errs.New("invalid --start-date %q: %v", startStr, err)
	}
	end, err := time.ParseInLocation("2006-01-02", endStr, time.UTC)
	if err != nil {
		return time.Time{}, time.Time{}, false, errs.New("invalid --end-date %q: %v", endStr, err)
	}
	if end.Before(start) {
		return time.Time{}, time.Time{}, false, errs.New("--end-date %q must be on or after --start-date %q", endStr, startStr)
	}
	endExclusive = end.AddDate(0, 0, 1)

	periodStart, periodEndExclusive := period.StartDate(), period.EndDateExclusive()
	if start.Before(periodStart) || endExclusive.After(periodEndExclusive) {
		return time.Time{}, time.Time{}, false, errs.New("--start-date %q and --end-date %q must be inside the --period %s (%s..%s)",
			startStr, endStr, period.String(),
			periodStart.Format("2006-01-02"), periodEndExclusive.AddDate(0, 0, -1).Format("2006-01-02"))
	}

	return start, endExclusive, true, nil
}

// runWithOutput invokes fn with the destination writer. When output is empty the
// data is written to stdout, otherwise it is written atomically to the named file.
func runWithOutput(output string, fn func(io.Writer) error) (err error) {
	if output == "" {
		return fn(os.Stdout)
	}
	outputTmp := output + ".tmp"
	file, err := os.Create(outputTmp)
	if err != nil {
		return errs.New("unable to create temporary output file: %v", err)
	}
	err = errs.Combine(err, fn(file))
	err = errs.Combine(err, file.Close())
	if err == nil {
		err = errs.Combine(err, os.Rename(outputTmp, output))
	}
	if err != nil {
		return errs.Combine(err, os.Remove(outputTmp))
	}
	return err
}

// runWithOutputs invokes fn with a destination writer for each output. Empty
// outputs are written to stdout, named outputs are collected in temporary files
// which are only moved into place once fn returned and every file was written
// successfully, so a failure cannot leave one of the outputs behind on its own.
func runWithOutputs(outputs []string, fn func([]io.Writer) error) error {
	type target struct {
		tmp   string
		final string
		file  *os.File
	}

	var targets []target
	writers := make([]io.Writer, 0, len(outputs))

	discard := func(err error) error {
		for _, t := range targets {
			_ = t.file.Close()
			err = errs.Combine(err, os.Remove(t.tmp))
		}
		return err
	}

	for _, output := range outputs {
		if output == "" {
			writers = append(writers, os.Stdout)
			continue
		}
		outputTmp := output + ".tmp"
		file, err := os.Create(outputTmp)
		if err != nil {
			return discard(errs.New("unable to create temporary output file: %v", err))
		}
		targets = append(targets, target{tmp: outputTmp, final: output, file: file})
		writers = append(writers, file)
	}

	if err := fn(writers); err != nil {
		return discard(err)
	}

	// Close every file before renaming any of them, so that a write error is
	// still detected while all the outputs can be discarded together.
	var closing errs.Group
	for _, t := range targets {
		closing.Add(t.file.Close())
	}
	if err := closing.Err(); err != nil {
		return discard(err)
	}

	var renaming errs.Group
	for _, t := range targets {
		renaming.Add(os.Rename(t.tmp, t.final))
	}
	return renaming.Err()
}
