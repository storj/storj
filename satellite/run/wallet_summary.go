// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/compensation"
	"storj.io/storj/shared/modular"
)

// WalletSummaryConfig configures the compensation wallet-summary subcommand.
type WalletSummaryConfig struct {
	Dir    string `help:"directory containing per-satellite invoice and incompletepaystub CSVs" default:"."`
	Glob   string `help:"glob (relative to --dir) matching invoice files; each match must have a sibling <prefix>-incompletepaystubs.csv" default:"*-invoices.csv"`
	Output string `help:"destination CSV path; empty means stdout" default:""`
}

// WalletSummary is a subcommand that aggregates invoice and incompletepaystub
// CSVs across satellites into one row per wallet, reporting how much would be
// distributed now and how much is still held pending graceful exit.
type WalletSummary struct {
	log    *zap.Logger
	config *WalletSummaryConfig
	stop   *modular.StopTrigger
}

// NewWalletSummary creates a new WalletSummary command.
func NewWalletSummary(log *zap.Logger, config *WalletSummaryConfig, stop *modular.StopTrigger) *WalletSummary {
	return &WalletSummary{log: log, config: config, stop: stop}
}

// Run executes the wallet-summary aggregation.
func (w *WalletSummary) Run(ctx context.Context) (err error) {
	defer w.stop.Cancel()

	pattern := filepath.Join(w.config.Dir, w.config.Glob)
	invoicePaths, err := filepath.Glob(pattern)
	if err != nil {
		return errs.Wrap(err)
	}
	if len(invoicePaths) == 0 {
		return errs.New("no invoice files matched %q", pattern)
	}

	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			err = errs.Combine(err, c.Close())
		}
	}()

	reports := make([]compensation.SatelliteReport, 0, len(invoicePaths))
	for _, invoicePath := range invoicePaths {
		ipaystubPath := strings.TrimSuffix(invoicePath, "-invoices.csv") + "-incompletepaystubs.csv"
		if !strings.HasSuffix(invoicePath, "-invoices.csv") {
			return errs.New("invoice path %q does not end in -invoices.csv; cannot derive incompletepaystubs sibling", invoicePath)
		}

		invoiceFile, err := os.Open(invoicePath)
		if err != nil {
			return errs.Wrap(err)
		}
		closers = append(closers, invoiceFile)

		ipaystubFile, err := os.Open(ipaystubPath)
		if err != nil {
			return errs.New("opening incompletepaystubs sibling for %q: %w", invoicePath, err)
		}
		closers = append(closers, ipaystubFile)

		name := strings.TrimSuffix(filepath.Base(invoicePath), "-invoices.csv")
		w.log.Info("reading satellite report",
			zap.String("name", name),
			zap.String("invoices", invoicePath),
			zap.String("incompletepaystubs", ipaystubPath))

		reports = append(reports, compensation.SatelliteReport{
			Name:               name,
			Invoices:           invoiceFile,
			IncompletePaystubs: ipaystubFile,
		})
	}

	return runWithOutput(w.config.Output, func(out io.Writer) error {
		return compensation.SummarizeWallets(reports, out)
	})
}
