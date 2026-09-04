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
	Period        string `help:"pay period to generate invoices for, a UTC date formatted like YYYY-MM" required:"true"`
	Output        string `help:"destination of report output" default:""`
	SurgePercent  int64  `help:"surge percent for payments" default:"0"`
	RecentCutoff  bool   `help:"if true, use the 24h before the period end (instead of the period start) as the cutoff for the offline and graceful-exiting checks. A node whose last successful contact is in that 24h window is treated as offline for the entire period and forfeits owed/held/disposed payments (including withheld-amount disposal), and only nodes still exiting at that cutoff are flagged GracefulExiting" default:"false"`
	Exclude       string `help:"Codes to be excluded from the final report, comma-separated" default:""`
	Cache         bool   `help:"preload per-node totals with one aggregate query instead of one query per node" default:"false"`
	StartDate     string `help:"optional partial-period start date (YYYY-MM-DD, inclusive). Must be set together with end-date. Overrides the period's month boundaries for usage aggregation and offline/DQ/GE/withholding checks. The paystub Period identifier still comes from --period, so only ONE partial run per --period may be recorded: record-period replaces the paystub on (period, node_id) and a second partial run would drop the first one's amounts from the lifetime totals. The range need not fall inside --period, so a range crossing a month boundary can be billed in one run, but then make sure the days outside --period were not already billed under their own period, as those paystubs have different period keys and will not collide. The usage aggregated over the range is exact, but the withholding tier and the disqualification cut-off are evaluated once, at the range end, so a range containing a tier step or a disqualification classifies its earlier days by the state at its end; the affected nodes are listed in a warning." default:""`
	EndDate       string `help:"optional partial-period end date (YYYY-MM-DD, inclusive). See --start-date." default:""`
	GenesisPeriod string `help:"optional genesis pay period (YYYY-MM, inclusive). When set, paystubs of earlier periods are ignored when summing held/disposed/paid/distributed totals; nodes with no matching paystubs are treated as zero. Filtering is on the paystub period, not on when the row was written, so the result does not change if an old period is re-recorded." default:""`
}

// RecordPeriodConfig configures the compensation-record-period subcommand.
type RecordPeriodConfig struct {
	PaystubsCSV string `help:"path to the paystubs CSV to record" required:"true"`
	PaymentsCSV string `help:"path to the payments CSV to record" required:"true"`
}

// RecordPaystubsConfig configures the compensation-record-paystubs subcommand.
type RecordPaystubsConfig struct {
	PaystubsCSV string `help:"path to the paystubs CSV to record, in either the finalized paystubs or the incomplete paystubs (prepare output) format" required:"true"`
	LegalHold   bool   `help:"zero out the paid and disposed amounts before recording, so nothing becomes available to the operator and the withheld escrow is not consumed. The rest of the paystub, including the owed and held amounts, is recorded as it is" default:"false"`
	Overwrite   bool   `help:"allow replacing already recorded paystubs that have a payout (a distributed amount or a recorded payment) against them. Without it such a file is refused, since replacing those rows makes the next prepare pay the nodes a second time" default:"false"`
}

// RecordOneOffPaymentsConfig configures the compensation-record-one-off-payments subcommand.
type RecordOneOffPaymentsConfig struct {
	PaymentsCSV string `help:"path to the payments CSV to record" required:"true"`
}

// RecordPaymentsConfig configures the compensation-record-payments subcommand.
type RecordPaymentsConfig struct {
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

	rangeStart, rangeEndExclusive, partial, err := parsePartialRange(g.config.StartDate, g.config.EndDate)
	if err != nil {
		return currency.Zero, 0, err
	}

	// endExclusive is the end of the range the statements are generated for: the
	// end of the month, or the end of the partial range when one is given.
	endExclusive := period.EndDateExclusive()

	var genesisPeriod *compensation.Period
	if g.config.GenesisPeriod != "" {
		parsed, err := compensation.PeriodFromString(g.config.GenesisPeriod)
		if err != nil {
			return currency.Zero, 0, errs.New("invalid --genesis-period %q: %v", g.config.GenesisPeriod, err)
		}
		genesisPeriod = &parsed
		g.log.Info("Ignoring paystubs of earlier periods when summing totals",
			zap.String("genesis_period", genesisPeriod.String()),
		)
	}

	var rangeEscapesPeriod bool
	if partial {
		endExclusive = rangeEndExclusive
		periodInfo.StartDateOverride = &rangeStart
		periodInfo.EndDateExclusiveOverride = &rangeEndExclusive
		g.log.Info("Generating invoices for partial period",
			zap.String("period", period.String()),
			zap.Time("start", rangeStart),
			zap.Time("end_exclusive", rangeEndExclusive),
		)
		// The paystub record-period writes is keyed on (period, node_id) and
		// replaced on conflict, so ANY other run recording the same --period
		// overwrites this one's held/owed/paid amounts, dropping them from the
		// lifetime totals that later withholding and disposal calculations read
		// back. That is true of every partial run, including two disjoint
		// in-period ranges, so this is not conditional on containment.
		g.log.Warn("Only ONE partial run per --period may be recorded; make sure no other run records this period",
			zap.String("period", period.String()),
		)
		rangeEscapesPeriod = rangeStart.Before(period.StartDate()) || rangeEndExclusive.After(period.EndDateExclusive())
		if rangeEscapesPeriod {
			// The days outside --period are billed under this period's key, so
			// their paystub does not collide with the one of the period they
			// belong to and nothing downstream detects the overlap: they are
			// paid twice if that period was billed already.
			g.log.Warn("Range extends outside the --period; make sure the days outside it were not billed under their own period already",
				zap.String("period", period.String()),
				zap.Time("period_start", period.StartDate()),
				zap.Time("period_end_exclusive", period.EndDateExclusive()),
			)
		}
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
		totalsCache, err = g.db.Compensation().QueryAllTotalAmounts(ctx, genesisPeriod)
		if err != nil {
			return currency.Zero, 0, errs.New("failed to preload totals cache: %+v", err)
		}
		g.log.Info("Loaded totals cache", zap.Int("nodes", len(totalsCache)))
	}

	invoices := make([]compensation.Invoice, 0, len(allNodes))
	progressCounter := mon.Counter("progress")
	for _, node := range allNodes {
		progressCounter.Inc(1)
		// QueryAllTotalAmounts covers every node with at least one paystub, so
		// with --cache a node missing from the aggregate genuinely has zero
		// totals and needs no per-node query.
		totalAmounts := totalsCache[node.Id]
		if totalsCache == nil {
			totalAmounts, err = g.db.Compensation().QueryTotalAmounts(ctx, node.Id, genesisPeriod)
			if err != nil {
				return currency.Zero, 0, err
			}
		}

		// the zero value of period usage is acceptable for if the node does not have
		// any usage for the period.
		nodeInfo, nodeLastIP, err := nodeInfoFromDossier(node, periodUsageByNode[node.Id], totalAmounts)
		if err != nil {
			return currency.Zero, 0, err
		}

		invoice := compensation.Invoice{
			Period:             period,
			NodeID:             compensation.NodeID(node.Id),
			NodeWallet:         node.Operator.Wallet,
			NodeWalletFeatures: node.Operator.WalletFeatures,
			NodeLastIP:         nodeLastIP,
		}

		if err := invoice.MergeNodeInfo(nodeInfo); err != nil {
			return currency.Zero, 0, err
		}
		invoices = append(invoices, invoice)
		periodInfo.Nodes = append(periodInfo.Nodes, nodeInfo)
	}

	if rangeEscapesPeriod {
		tierStepped, disqualifiedInRange := classificationBoundariesInRange(periodInfo.Nodes, g.comp.WithheldPercents, rangeStart, rangeEndExclusive)
		if len(tierStepped) > 0 {
			g.log.Warn("Nodes whose withholding tier steps inside the range: their days before the step are withheld at the tier of the range end, not at the one the period those days belong to would have used",
				zap.Int("nodes", len(tierStepped)),
				zap.Strings("node_ids", sampleNodeIDs(tierStepped)),
			)
		}
		if len(disqualifiedInRange) > 0 {
			g.log.Warn("Nodes disqualified inside the range: their days before the disqualification are zeroed, whereas billing those days under their own period would have paid them",
				zap.Int("nodes", len(disqualifiedInRange)),
				zap.Strings("node_ids", sampleNodeIDs(disqualifiedInRange)),
			)
		}
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

// nodeInfoFromDossier converts a node dossier, its usage over the invoiced
// range and its lifetime paystub totals into the compensation input. It also
// returns the node's last known IP, without the port, for OFAC screening.
func nodeInfoFromDossier(node *overlay.NodeDossier, usage accounting.StorageNodePeriodUsage, totalAmounts compensation.TotalAmounts) (compensation.NodeInfo, string, error) {
	// GracefulExit means the node left and earned its withheld amount back, so
	// it is only set for a successful exit. ExitFinished is set either way,
	// since a failed exit takes the node out of the network just the same.
	var gracefulExit *time.Time
	if node.ExitStatus.ExitSuccess {
		gracefulExit = node.ExitStatus.ExitFinishedAt
	}

	var nodeLastIP string
	if node.LastIPPort != "" {
		ip, _, err := net.SplitHostPort(node.LastIPPort)
		if err != nil {
			return compensation.NodeInfo{}, "", errs.New("unable to split node %q last ip:port %q", node.Id, node.LastIPPort)
		}
		nodeLastIP = ip
	}

	return compensation.NodeInfo{
		ID:                 node.Id,
		CreatedAt:          node.CreatedAt,
		LastContactSuccess: node.Reputation.LastContactSuccess,
		Disqualified:       node.Disqualified,
		GracefulExit:       gracefulExit,
		ExitFinished:       node.ExitStatus.ExitFinishedAt,
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
	}, nodeLastIP, nil
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

// RecordPaystubs is a tool subcommand that records storage node paystubs
// without any payment. Unlike record-period it accepts both the finalized
// paystubs and the incomplete paystubs written by prepare. An incomplete
// paystub is recorded with a zero distributed amount, since nothing proves a
// payout was executed for it; a finalized one keeps the distributed amount,
// which Finalize only writes for a paystub covered by a receipt.
type RecordPaystubs struct {
	log    *zap.Logger
	db     satellite.DB
	config *RecordPaystubsConfig
	stop   *modular.StopTrigger
}

// NewRecordPaystubs creates a new RecordPaystubs command.
func NewRecordPaystubs(log *zap.Logger, db satellite.DB, config *RecordPaystubsConfig, stop *modular.StopTrigger) *RecordPaystubs {
	return &RecordPaystubs{
		log:    log,
		db:     db,
		config: config,
		stop:   stop,
	}
}

// Run records the paystubs.
func (r *RecordPaystubs) Run(ctx context.Context) (err error) {
	defer r.stop.Cancel()

	paystubs, err := compensation.LoadAnyPaystubs(r.config.PaystubsCSV)
	if err != nil {
		return err
	}

	if r.config.LegalHold {
		// A distributed amount in the file says the money already reached the
		// operator, so there is nothing left to withhold and zeroing paid next
		// to it would break the TotalPaid >= TotalDistributed invariant.
		var distributed int
		for _, paystub := range paystubs {
			if paystub.Distributed.Value() != 0 {
				distributed++
			}
		}
		if distributed > 0 {
			return errs.New("refusing to put %d of the %d paystubs on legal hold: they record a distributed amount, so the payout was already executed for them", distributed, len(paystubs))
		}
		for i := range paystubs {
			paystubs[i] = paystubs[i].LegalHold()
		}
	}

	if err := r.db.CheckVersion(ctx); err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	if err := r.checkConflicts(ctx, paystubs); err != nil {
		return err
	}

	if err := r.db.Compensation().RecordPaystubs(ctx, paystubs); err != nil {
		return err
	}

	r.log.Info("Recorded paystubs",
		zap.Int("paystubs", len(paystubs)),
		zap.Bool("legal_hold", r.config.LegalHold),
	)
	return nil
}

// checkConflicts refuses the run when it would replace an already recorded
// paystub that has a payout against it, unless --overwrite says otherwise.
// RecordPaystubs replaces the paystub row of a (period, node) but leaves the
// storagenode_payments rows behind, so overwriting a distributed amount with a
// smaller one makes the next prepare carry the difference over and pay the node
// again, with no trace of what the row looked like before.
func (r *RecordPaystubs) checkConflicts(ctx context.Context, paystubs []compensation.Paystub) error {
	conflicts, err := r.db.Compensation().QueryPaystubConflicts(ctx, paystubs)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}

	type key struct {
		period compensation.Period
		nodeID compensation.NodeID
	}
	incoming := make(map[key]currency.MicroUnit, len(paystubs))
	for _, paystub := range paystubs {
		incoming[key{paystub.Period, paystub.NodeID}] = paystub.Distributed
	}

	var lossy []compensation.PaystubConflict
	var lost int64
	for _, conflict := range conflicts {
		if !conflict.PaidOut() {
			continue
		}
		replacement := incoming[key{conflict.Period, compensation.NodeID(conflict.NodeID)}]
		if drop := conflict.Distributed.Value() - replacement.Value(); drop > 0 {
			lost += drop
		}
		lossy = append(lossy, conflict)
	}

	if len(lossy) > 0 && !r.config.Overwrite {
		return errs.New("refusing to replace %d of the %d paystubs: they are already recorded with a payout against them (%s of distributed amount would be dropped, e.g. node %s in %s), which makes the next prepare pay those nodes a second time. Pass --overwrite if the payout really was not executed",
			len(lossy), len(paystubs), currency.NewMicroUnit(lost).FloatString(), lossy[0].NodeID, lossy[0].Period)
	}

	log := r.log.Info
	if len(lossy) > 0 {
		log = r.log.Warn
	}
	log("Replacing already recorded paystubs",
		zap.Int("paystubs", len(conflicts)),
		zap.Int("with_payout", len(lossy)),
		zap.String("distributed_dropped", currency.NewMicroUnit(lost).FloatString()),
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

// RecordPayments is a tool subcommand that records the executed payments of a
// pay period and distributes them on the paystubs the payments belong to.
type RecordPayments struct {
	log    *zap.Logger
	db     satellite.DB
	config *RecordPaymentsConfig
	stop   *modular.StopTrigger
}

// NewRecordPayments creates a new RecordPayments command.
func NewRecordPayments(log *zap.Logger, db satellite.DB, config *RecordPaymentsConfig, stop *modular.StopTrigger) *RecordPayments {
	return &RecordPayments{
		log:    log,
		db:     db,
		config: config,
		stop:   stop,
	}
}

// Run records the payments and updates the distributed amount of their paystubs.
func (r *RecordPayments) Run(ctx context.Context) (err error) {
	defer r.stop.Cancel()

	payments, err := compensation.LoadPayments(r.config.PaymentsCSV)
	if err != nil {
		return err
	}

	if err := r.db.CheckVersion(ctx); err != nil {
		return errs.New("Error checking version for satellitedb: %+v", err)
	}

	if err := r.db.Compensation().RecordPaymentsWithDistribution(ctx, payments); err != nil {
		return err
	}

	var total int64
	for _, payment := range payments {
		total += payment.Amount.Value()
	}

	r.log.Info("Recorded payments",
		zap.Int("payments", len(payments)),
		zap.String("total", currency.NewMicroUnit(total).FloatString()),
	)
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
// The range is not required to be contained in --period: the usage query is a
// plain date-range query, so the usage aggregated over a range crossing a month
// boundary is exact. The date-based classification is not: GenerateStatements
// evaluates the withholding tier and the disqualification cut-off once, at the
// range end, so every day of the range is classified as its last day is. The
// caller reports the nodes that difference applies to (see
// classificationBoundariesInRange).
//
// The caller warns rather than refusing, because the paystub written later by
// record-period is keyed on (period, node_id) and replaced on conflict: a
// second run for the same --period silently overwrites the paystub of the first
// one, dropping its held/owed/paid amounts from the lifetime totals that later
// withholding and disposal calculations read back. A range escaping --period
// additionally risks paying the days outside it twice, since they are recorded
// under this period's key and so never collide with the paystub of the period
// they belong to.
func parsePartialRange(startStr, endStr string) (start, endExclusive time.Time, partial bool, err error) {
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

	return start, endExclusive, true, nil
}

// classificationBoundariesInRange returns the nodes whose withholding tier
// steps, or whose disqualification falls, between the first day of the range
// and its end, i.e. the nodes for which the single classification
// GenerateStatements derives at the range end does not hold for the whole
// range.
//
// GenerateStatements evaluates NodeWithheldPercent and the Disqualified
// cut-off once, against the end of the range, so every day of the range is
// classified as its last day is. For a range inside one month that matches
// what the whole-month run does anyway, but a range crossing a month boundary
// collapses two months into one statement: the earlier month's days are
// withheld at the tier the node reaches by the range end, and are zeroed by a
// disqualification that had not happened yet when they were earned. Billing
// those days under their own --period would have classified them on that
// month's end date instead, so the operator needs to know which nodes the
// difference applies to.
//
// withheldPercents may be nil, in which case the defaults GenerateStatements
// falls back to are used. The tiers are compared as of the first day of the
// range: a step on that day is already accounted for by every classification
// of the range, so it is not a boundary the range spans. The comparison is on
// the resulting percent, not on the tier index, so crossing a month boundary
// inside a run of equal percents (as the default schedule has) withholds the
// same amount either way and is not reported.
func classificationBoundariesInRange(nodes []compensation.NodeInfo, withheldPercents []int, rangeStart, rangeEndExclusive time.Time) (tierStepped, disqualified []storj.NodeID) {
	if withheldPercents == nil {
		withheldPercents = compensation.DefaultWithheldPercents
	}
	firstDayEndExclusive := rangeStart.AddDate(0, 0, 1)
	for _, node := range nodes {
		startPercent, startInWithholding := compensation.NodeWithheldPercent(withheldPercents, node.CreatedAt, firstDayEndExclusive)
		endPercent, endInWithholding := compensation.NodeWithheldPercent(withheldPercents, node.CreatedAt, rangeEndExclusive)
		if startPercent != endPercent || startInWithholding != endInWithholding {
			tierStepped = append(tierStepped, node.ID)
		}

		// A gracefully exited node is exempt from the zeroing (see
		// GenerateStatements), so its disqualification date makes no
		// difference to the amounts.
		gracefullyExited := node.GracefulExit != nil && node.GracefulExit.Before(rangeEndExclusive)
		if node.Disqualified != nil && !gracefullyExited &&
			node.Disqualified.Before(rangeEndExclusive) && !node.Disqualified.Before(firstDayEndExclusive) {
			disqualified = append(disqualified, node.ID)
		}
	}
	return tierStepped, disqualified
}

// maxSampledNodeIDs bounds the node IDs a single warning lists, so that a range
// spanning a tier step of a whole node cohort does not emit a log line with
// tens of thousands of IDs in it. The warnings report the full count alongside.
const maxSampledNodeIDs = 20

// sampleNodeIDs formats at most maxSampledNodeIDs of the given IDs, appending a
// marker when the list was cut short.
func sampleNodeIDs(ids []storj.NodeID) []string {
	sampled := make([]string, 0, min(len(ids), maxSampledNodeIDs)+1)
	for _, id := range ids {
		if len(sampled) == maxSampledNodeIDs {
			sampled = append(sampled, "...")
			break
		}
		sampled = append(sampled, id.String())
	}
	return sampled
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
