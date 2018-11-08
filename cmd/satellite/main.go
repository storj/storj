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

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	// "storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/auth/grpcauth"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/statdb"
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
		Use:   "setup",
		Short: "Create config files",
		RunE:  cmdSetup,
	}
	diagCmd = &cobra.Command{
		Use:   "diag",
		Short: "Diagnostic Tool support",
		RunE:  cmdDiag,
	}

	runCfg struct {
		Identity  provider.IdentityConfig
		Kademlia  kademlia.Config
		PointerDB pointerdb.Config
		// Checker     checker.Config
		// Repairer    repairer.Config
		Overlay overlay.Config
		StatDB  statdb.Config
		// RepairQueue   queue.Config
		// RepairChecker checker.Config
		// Repairer      repairer.Config
		// Audit audit.Config
	}
	setupCfg struct {
		BasePath  string `default:"$CONFDIR" help:"base path for setup"`
		CA        provider.CASetupConfig
		Identity  provider.IdentitySetupConfig
		Overwrite bool `default:"false" help:"whether to overwrite pre-existing configuration files"`
	}
	diagCfg struct {
		BasePath string `default:"$CONFDIR" help:"base path for setup"`
	}

	defaultConfDir = "$HOME/.storj/satellite"
	defaultDiagDir = "postgres://postgres@localhost/pointerdb?sslmode=disable"
)

func init() {
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(diagCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(diagCmd.Flags(), &diagCfg, cfgstruct.ConfDir(defaultDiagDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	return runCfg.Identity.Run(
		process.Ctx(cmd),
		grpcauth.NewAPIKeyInterceptor(),
		runCfg.Kademlia,
		runCfg.PointerDB,
		runCfg.Overlay,
		runCfg.StatDB,
		// runCfg.Audit,
	)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupCfg.BasePath, err = filepath.Abs(setupCfg.BasePath)
	if err != nil {
		return err
	}

	_, err = os.Stat(setupCfg.BasePath)
	if !setupCfg.Overwrite && err == nil {
		fmt.Println("An satellite configuration already exists. Rerun with --overwrite")
		return nil
	} else if setupCfg.Overwrite && err == nil {
		fmt.Println("overwriting existing satellite config")
		err = os.RemoveAll(setupCfg.BasePath)
		if err != nil {
			return err
		}
	}

	err = os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	// TODO: handle setting base path *and* identity file paths via args
	// NB: if base path is set this overrides identity and CA path options
	if setupCfg.BasePath != defaultConfDir {
		setupCfg.CA.CertPath = filepath.Join(setupCfg.BasePath, "ca.cert")
		setupCfg.CA.KeyPath = filepath.Join(setupCfg.BasePath, "ca.key")
		setupCfg.Identity.CertPath = filepath.Join(setupCfg.BasePath, "identity.cert")
		setupCfg.Identity.KeyPath = filepath.Join(setupCfg.BasePath, "identity.key")
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
		filepath.Join(setupCfg.BasePath, "config.yaml"), o)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	// open the psql db
	dbpath := diagCfg.BasePath
	s, err := bwagreement.NewServer("postgres", dbpath, zap.NewNop())
	if err != nil {
		fmt.Println("Storagenode database couldnt open:", dbpath)
		return err
	}
	//get all bandwidth aggrements rows already ordered
	baRows, err := s.GetBandwidthAllocations(context.Background())
	if err != nil {
		fmt.Printf("error reading satellite database %v: %v\n", dbpath, err)
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
	summaries := make(map[string]*UplinkSummary)
	uplinkIDs := []string{}

	for _, baRow := range baRows {
		// deserializing rbad you get payerbwallocation, total & storage node id
		rbad := &pb.RenterBandwidthAllocation_Data{}
		if err := proto.Unmarshal(baRow.Data, rbad); err != nil {
			return err
		}

		// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
		pbad := &pb.PayerBandwidthAllocation_Data{}
		if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
			return err
		}

		uplinkID := string(pbad.GetUplinkId())
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
	sort.Strings(uplinkIDs)
	for _, uplinkID := range uplinkIDs {
		summary := summaries[uplinkID]
		fmt.Fprint(w, uplinkID, "\t", summary.TotalBytes, "\t", summary.TotalTransactions, "\t", summary.PutActionCount, "\t", summary.GetActionCount, "\t\n")
	}

	// display the data
	err = w.Flush()
	return err
}

func main() {
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
