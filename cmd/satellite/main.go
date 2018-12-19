// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/discovery"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/satellitedb"
)

var (
	rootCmd = &cobra.Command{
		Use:   "satellite",
		Short: "Satellite",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the satellite",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	diagCmd = &cobra.Command{
		Use:   "diag",
		Short: "Diagnostic Tool support",
		RunE:  cmdDiag,
	}
	qdiagCmd = &cobra.Command{
		Use:   "qdiag",
		Short: "Repair Queue Diagnostic Tool support",
		RunE:  cmdQDiag,
	}

	runCfg struct {
		Identity    provider.IdentityConfig
		Kademlia    kademlia.Config
		PointerDB   pointerdb.Config
		Overlay     overlay.Config
		Checker     checker.Config
		Repairer    repairer.Config
		Audit       audit.Config
		BwAgreement bwagreement.Config
		Database    string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
		Discovery   discovery.Config
	}
	setupCfg struct {
		CA        provider.CASetupConfig
		Identity  provider.IdentitySetupConfig
		Overwrite bool `default:"false" help:"whether to overwrite pre-existing configuration files"`
	}
	diagCfg struct {
		Database string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
	}
	qdiagCfg struct {
		Database   string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
		QListLimit int    `help:"maximum segments that can be requested" default:"1000"`
	}

	defaultConfDir string
	confDir        *string
)

func init() {
	defaultConfDir = fpath.ApplicationDir("storj", "satellite")

	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	confDir = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for satellite configuration")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(diagCmd)
	rootCmd.AddCommand(qdiagCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(diagCmd.Flags(), &diagCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(qdiagCmd.Flags(), &qdiagCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	database, err := satellitedb.New(runCfg.Database)
	if err != nil {
		return errs.New("Error starting master database on satellite: %+v", err)
	}

	err = database.CreateTables()
	if err != nil {
		return errs.New("Error creating tables for master database on satellite: %+v", err)
	}

	//nolint ignoring context rules to not create cyclic dependency, will be removed later
	ctx = context.WithValue(ctx, "masterdb", database)

	return runCfg.Identity.Run(
		ctx,
		grpcauth.NewAPIKeyInterceptor(),
		runCfg.Kademlia,
		runCfg.Overlay,
		runCfg.PointerDB,
		runCfg.Checker,
		runCfg.Repairer,
		runCfg.Audit,
		runCfg.BwAgreement,
		runCfg.Discovery,
	)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(*confDir)
	if err != nil {
		return err
	}

	valid, err := fpath.IsValidSetupDir(setupDir)
	if !setupCfg.Overwrite && !valid {
		fmt.Printf("satellite configuration already exists (%v). rerun with --overwrite\n", setupDir)
		return nil
	} else if setupCfg.Overwrite && err == nil {
		fmt.Println("overwriting existing satellite config")
		err = os.RemoveAll(setupDir)
		if err != nil {
			return err
		}
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	// TODO: handle setting base path *and* identity file paths via args
	// NB: if base path is set this overrides identity and CA path options
	if setupDir != defaultConfDir {
		setupCfg.CA.CertPath = filepath.Join(setupDir, "ca.cert")
		setupCfg.CA.KeyPath = filepath.Join(setupDir, "ca.key")
		setupCfg.Identity.CertPath = filepath.Join(setupDir, "identity.cert")
		setupCfg.Identity.KeyPath = filepath.Join(setupDir, "identity.key")
	}
	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
	if err != nil {
		return err
	}

	o := map[string]interface{}{
		"identity.cert-path": setupCfg.Identity.CertPath,
		"identity.key-path":  setupCfg.Identity.KeyPath,
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupDir, "config.yaml"), o)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	database, err := satellitedb.New(diagCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err := database.Close()
		if err != nil {
			fmt.Printf("error closing connection to master database on satellite: %+v\n", err)
		}
	}()

	//get all bandwidth agreements rows already ordered
	baRows, err := database.BandwidthAgreement().GetAgreements(context.Background())
	if err != nil {
		fmt.Printf("error reading satellite database %v: %v\n", diagCfg.Database, err)
		return err
	}

	// Agreement is a struct that contains a bandwidth agreement and the associated signature
	type UplinkSummary struct {
		TotalBytes        int64
		PutActionCount    int64
		GetActionCount    int64
		TotalTransactions int64
		// additional attributes add here ...
	}

	// attributes per uplinkid
	summaries := make(map[storj.NodeID]*UplinkSummary)
	uplinkIDs := storj.NodeIDList{}

	for _, baRow := range baRows {
		// deserializing rbad you get payerbwallocation, total & storage node id
		rbad := &pb.RenterBandwidthAllocation_Data{}
		if err := proto.Unmarshal(baRow.Agreement, rbad); err != nil {
			return err
		}

		// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
		pbad := &pb.PayerBandwidthAllocation_Data{}
		if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
			return err
		}

		uplinkID := pbad.UplinkId
		summary, ok := summaries[uplinkID]
		if !ok {
			summaries[uplinkID] = &UplinkSummary{}
			uplinkIDs = append(uplinkIDs, uplinkID)
			summary = summaries[uplinkID]
		}

		// fill the summary info
		summary.TotalBytes += rbad.GetTotal()
		summary.TotalTransactions++
		if pbad.GetAction() == pb.PayerBandwidthAllocation_PUT {
			summary.PutActionCount++
		} else {
			summary.GetActionCount++
		}
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "UplinkID\tTotal\t# Of Transactions\tPUT Action\tGET Action\t")

	// populate the row fields
	sort.Sort(uplinkIDs)
	for _, uplinkID := range uplinkIDs {
		summary := summaries[uplinkID]
		fmt.Fprint(w, uplinkID, "\t", summary.TotalBytes, "\t", summary.TotalTransactions, "\t", summary.PutActionCount, "\t", summary.GetActionCount, "\t\n")
	}

	// display the data
	return w.Flush()
}

func cmdQDiag(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	// open the master db
	database, err := satellitedb.New(qdiagCfg.Database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err := database.Close()
		if err != nil {
			fmt.Printf("error closing connection to master database on satellite: %+v\n", err)
		}
	}()

	list, err := database.RepairQueue().Peekqueue(ctx, qdiagCfg.QListLimit)
	if err != nil {
		return err
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "Path\tLost Pieces\t")

	// populate the row fields
	for _, v := range list {
		fmt.Fprint(w, v.GetPath(), "\t", v.GetLostPieces(), "\t")
	}

	// display the data
	return w.Flush()
}

func main() {
	process.Exec(rootCmd)
}
