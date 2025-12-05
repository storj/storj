// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

type diagCfg struct {
	storagenode.Config

	DiagDir string `internal:"true"`
}

func newDiagCmd(f *Factory) *cobra.Command {
	var diagCfg diagCfg
	cmd := &cobra.Command{
		Use:   "diag",
		Short: "Diagnostic Tool support",
		RunE: func(cmd *cobra.Command, args []string) error {
			diagDir, err := filepath.Abs(f.ConfDir)
			if err != nil {
				return err
			}
			diagCfg.DiagDir = diagDir
			return cmdDiag(cmd, &diagCfg)
		},
		Annotations: map[string]string{"type": "helper"},
	}

	process.Bind(cmd, &diagCfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func cmdDiag(cmd *cobra.Command, cfg *diagCfg) (err error) {
	ctx, _ := process.Ctx(cmd)

	// check if the directory exists
	_, err = os.Stat(cfg.DiagDir)
	if err != nil {
		fmt.Println("storage node directory doesn't exist", cfg.DiagDir)
		return err
	}

	db, err := storagenodedb.OpenExisting(ctx, zap.L().Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storage node: %v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	summaries, err := db.Bandwidth().SummaryBySatellite(ctx, time.Time{}, time.Now())
	if err != nil {
		fmt.Printf("unable to get bandwidth summary: %v\n", err)
		return err
	}

	satellites := storj.NodeIDList{}
	for id := range summaries {
		satellites = append(satellites, id)
	}
	sort.Sort(satellites)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight|tabwriter.Debug)
	defer func() { err = errs.Combine(err, w.Flush()) }()

	_, _ = fmt.Fprint(w, "Satellite\tTotal\tPut\tGet\tDelete\tAudit Get\tRepair Get\tRepair Put\n")

	for _, id := range satellites {
		summary := summaries[id]
		_, _ = fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			id,
			memory.Size(summary.Total()),
			memory.Size(summary.Put),
			memory.Size(summary.Get),
			memory.Size(summary.Delete),
			memory.Size(summary.GetAudit),
			memory.Size(summary.GetRepair),
			memory.Size(summary.PutRepair),
		)
	}

	return nil
}
