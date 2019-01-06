// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"text/tabwriter"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
)

// StorageNode defines storage node runtime configuration
type StorageNode struct {
	Server   server.Config
	Kademlia kademlia.StorageNodeConfig
	Storage  psserver.Config
}

// SetupStorageNode defines storage node setup configuration
type SetupStorageNode struct {
	CA        identity.CASetupConfig
	Identity  identity.SetupConfig
	Signer    certificates.CertSigningConfig
	Overwrite bool `default:"false" help:"whether to overwrite pre-existing configuration files"`

	StorageNode
}

var (
	rootCmd = &cobra.Command{
		Use:   "storagenode",
		Short: "StorageNode",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the storagenode",
		RunE:  cmdRun,
	}
	setupCmd = &cobra.Command{
		Use:         "setup",
		Short:       "Create config files",
		RunE:        cmdSetup,
		Annotations: map[string]string{"type": "setup"},
	}
	configCmd = &cobra.Command{
		Use:         "config",
		Short:       "Edit config files",
		RunE:        cmdConfig,
		Annotations: map[string]string{"type": "setup"},
	}
	diagCmd = &cobra.Command{
		Use:   "diag",
		Short: "Diagnostic Tool support",
		RunE:  cmdDiag,
	}

	runCfg   StorageNode
	setupCfg SetupStorageNode

	diagCfg struct {
	}

	defaultConfDir string
	defaultDiagDir string
	confDir        *string
)

const (
	defaultServerAddr    = ":28967"
	defaultSatteliteAddr = "127.0.0.1:7778"
)

func init() {
	defaultConfDir = fpath.ApplicationDir("storj", "storagenode")

	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	confDir = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for storagenode configuration")

	defaultDiagDir = filepath.Join(defaultConfDir, "storage")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(diagCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(diagCmd.Flags(), &diagCfg, cfgstruct.ConfDir(defaultDiagDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	operatorConfig := runCfg.Kademlia.Operator
	if err := isOperatorEmailValid(operatorConfig.Email); err != nil {
		zap.S().Warn(err)
	} else {
		zap.S().Info("Operator email: ", operatorConfig.Email)
	}
	if err := isOperatorWalletValid(operatorConfig.Wallet); err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Operator wallet: ", operatorConfig.Wallet)
	}

	return runCfg.Server.Run(process.Ctx(cmd), nil, runCfg.Kademlia, runCfg.Storage)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(*confDir)
	if err != nil {
		return err
	}

	valid, err := fpath.IsValidSetupDir(setupDir)
	if !setupCfg.Overwrite && !valid {
		return fmt.Errorf("storagenode configuration already exists (%v). Rerun with --overwrite", setupDir)
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

	// TODO: this is only applicable once we stop deleting the entire config dir on overwrite
	// (see https://storjlabs.atlassian.net/browse/V3-1013)
	// (see https://storjlabs.atlassian.net/browse/V3-949)
	if setupCfg.Overwrite {
		setupCfg.CA.Overwrite = true
		setupCfg.Identity.Overwrite = true
	}
	setupCfg.CA.CertPath = filepath.Join(setupDir, "ca.cert")
	setupCfg.CA.KeyPath = filepath.Join(setupDir, "ca.key")
	setupCfg.Identity.CertPath = filepath.Join(setupDir, "identity.cert")
	setupCfg.Identity.KeyPath = filepath.Join(setupDir, "identity.key")

	if setupCfg.Signer.AuthToken != "" && setupCfg.Signer.Address != "" {
		err = setupCfg.Signer.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
		if err != nil {
			zap.S().Warn(err)
		}
	} else {
		err = identity.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
		if err != nil {
			return err
		}
	}

	overrides := map[string]interface{}{
		"identity.cert-path":                      setupCfg.Identity.CertPath,
		"identity.key-path":                       setupCfg.Identity.KeyPath,
		"identity.server.address":                 defaultServerAddr,
		"storage.path":                            filepath.Join(setupDir, "storage"),
		"kademlia.bootstrap-addr":                 defaultSatteliteAddr,
		"piecestore.agreementsender.overlay-addr": defaultSatteliteAddr,
	}

	return process.SaveConfig(cmd.Flags(), filepath.Join(setupDir, "config.yaml"), overrides)
}

func cmdConfig(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(*confDir)
	if err != nil {
		return err
	}
	//run setup if we can't access the config file
	conf := filepath.Join(setupDir, "config.yaml")
	if _, err := os.Stat(conf); err != nil {
		if err = cmdSetup(cmd, args); err != nil {
			return err
		}
	}
	return fpath.EditFile(conf)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	diagDir, err := filepath.Abs(*confDir)
	if err != nil {
		return err
	}

	// check if the directory exists
	_, err = os.Stat(diagDir)
	if err != nil {
		fmt.Println("Storagenode directory doesn't exist", diagDir)
		return err
	}

	// open the sql db
	dbpath := filepath.Join(diagDir, "storage", "piecestore.db")
	db, err := psdb.Open(context.Background(), "", dbpath)
	if err != nil {
		fmt.Println("Storagenode database couldnt open:", dbpath)
		return err
	}

	//get all bandwidth aggrements entries already ordered
	bwAgreements, err := db.GetBandwidthAllocations()
	if err != nil {
		fmt.Println("storage node 'bandwidth_agreements' table read error:", dbpath)
		return err
	}

	// Agreement is a struct that contains a bandwidth agreement and the associated signature
	type SatelliteSummary struct {
		TotalBytes        int64
		PutActionCount    int64
		GetActionCount    int64
		TotalTransactions int64
		// additional attributes add here ...
	}

	// attributes per satelliteid
	summaries := make(map[storj.NodeID]*SatelliteSummary)
	satelliteIDs := storj.NodeIDList{}

	for _, rbaVal := range bwAgreements {
		for _, rbaDataVal := range rbaVal {
			// deserializing rbad you get payerbwallocation, total & storage node id
			rbad := &pb.RenterBandwidthAllocation_Data{}
			if err := proto.Unmarshal(rbaDataVal.Agreement, rbad); err != nil {
				return err
			}

			// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
			pbad := &pb.PayerBandwidthAllocation_Data{}
			if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
				return err
			}

			summary, ok := summaries[pbad.SatelliteId]
			if !ok {
				summaries[pbad.SatelliteId] = &SatelliteSummary{}
				satelliteIDs = append(satelliteIDs, pbad.SatelliteId)
				summary = summaries[pbad.SatelliteId]
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
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "SatelliteID\tTotal\t# Of Transactions\tPUT Action\tGET Action\t")

	// populate the row fields
	sort.Sort(satelliteIDs)
	for _, satelliteID := range satelliteIDs {
		summary := summaries[satelliteID]
		fmt.Fprint(w, satelliteID, "\t", summary.TotalBytes, "\t", summary.TotalTransactions, "\t", summary.PutActionCount, "\t", summary.GetActionCount, "\t\n")
	}

	// display the data
	err = w.Flush()
	return err
}

func isOperatorEmailValid(email string) error {
	if email == "" {
		return fmt.Errorf("Operator mail address isn't specified")
	}
	return nil
}

func isOperatorWalletValid(wallet string) error {
	if wallet == "" {
		return fmt.Errorf("Operator wallet address isn't specified")
	}
	r := regexp.MustCompile("^0x[a-fA-F0-9]{40}$")
	if match := r.MatchString(wallet); !match {
		return fmt.Errorf("Operator wallet address isn't valid")
	}
	return nil
}

func main() {
	process.Exec(rootCmd)
}
