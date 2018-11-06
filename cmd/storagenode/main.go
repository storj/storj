// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	psserver "storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

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
		Identity provider.IdentityConfig
		Kademlia kademlia.Config
		Storage  psserver.Config
	}
	setupCfg struct {
		BasePath string `default:"$CONFDIR" help:"base path for setup"`
		CA       provider.CASetupConfig
		Identity provider.IdentitySetupConfig
	}
	diagCfg struct {
		BasePath string `default:"$CONFDIR" help:"base path for setup"`
	}

	defaultConfDir = "$HOME/.storj/storagenode"
	defaultDiagDir = "$HOME/.storj/capt/f37/data"
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
	return runCfg.Identity.Run(process.Ctx(cmd), nil, runCfg.Kademlia, runCfg.Storage)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupCfg.BasePath, err = filepath.Abs(setupCfg.BasePath)
	if err != nil {
		return err
	}

	err = os.MkdirAll(setupCfg.BasePath, 0700)
	if err != nil {
		return err
	}

	setupCfg.CA.CertPath = filepath.Join(setupCfg.BasePath, "ca.cert")
	setupCfg.CA.KeyPath = filepath.Join(setupCfg.BasePath, "ca.key")
	setupCfg.Identity.CertPath = filepath.Join(setupCfg.BasePath, "identity.cert")
	setupCfg.Identity.KeyPath = filepath.Join(setupCfg.BasePath, "identity.key")

	err = provider.SetupIdentity(process.Ctx(cmd), setupCfg.CA, setupCfg.Identity)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"identity.cert-path": setupCfg.Identity.CertPath,
		"identity.key-path":  setupCfg.Identity.KeyPath,
		"storage.path":       filepath.Join(setupCfg.BasePath, "storage"),
	}

	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupCfg.BasePath, "config.yaml"), overrides)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	diagCfg.BasePath, err = filepath.Abs(diagCfg.BasePath)
	if err != nil {
		return err
	}

	// check if the directory exists
	_, err = os.Stat(diagCfg.BasePath)
	if err != nil {
		fmt.Println("Storagenode directory doesn't exist", diagCfg.BasePath)
		return err
	}

	// open the sql db
	dbpath := filepath.Join(diagCfg.BasePath, "piecestore.db")
	db, err := psdb.Open(context.Background(), "", dbpath)
	if err != nil {
		fmt.Println("Storagenode database couldnt open:", dbpath)
		return err
	}

	// Agreement is a struct that contains a bandwidth agreement and the associated signature
	type SatAttributes struct {
		TotalBytes        int64
		PutActionCount    int64
		GetActionCount    int64
		TotalTransactions int64
		// additional attributes add here ...
	}

	//get all bandwidth aggrements entries already ordered
	bwAgreements, err := db.GetBandwidthAllocations()
	if err != nil {
		fmt.Println("bwallocation table database read error")
	}
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "SatelliteID\tTotal\t# Of Transactions\tPUT Action\tGET Action\t")

	// attributes per satelliteid
	satelliteID := make(map[string]SatAttributes)
	satAtt := SatAttributes{}
	var currSatID, lastSatID string

	for _, v := range bwAgreements {
		for _, j := range v {
			// deserializing rbad you get payerbwallocation, total & storage node id
			rbad := &pb.RenterBandwidthAllocation_Data{}
			if err := proto.Unmarshal(j.Agreement, rbad); err != nil {
				return err
			}
			total := rbad.GetTotal()

			// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
			pbad := &pb.PayerBandwidthAllocation_Data{}
			if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
				return err
			}
			currSatID = fmt.Sprintf("%s", pbad.GetSatelliteId())
			action := pbad.GetAction()

			if strings.Compare(currSatID, lastSatID) != 0 {
				// make an entry
				satAtt.TotalBytes = total
				if action == pb.PayerBandwidthAllocation_PUT {
					satAtt.PutActionCount++
				} else {
					satAtt.GetActionCount++
					fmt.Println(satAtt.GetActionCount)
				}
				satAtt.TotalTransactions++
				satelliteID[currSatID] = satAtt
				lastSatID = currSatID
			} else {
				//update the already existing entry
				for satIDKey, satIDVal := range satelliteID {
					if strings.Compare(satIDKey, currSatID) == 0 {
						satIDVal.TotalBytes = satIDVal.TotalBytes + total
						if action == pb.PayerBandwidthAllocation_PUT {
							satIDVal.PutActionCount++
						} else {
							satIDVal.GetActionCount++
						}
						satIDVal.TotalTransactions++
						satelliteID[satIDKey] = satIDVal
					}
				}
			}
		}
		fmt.Fprintln(w, currSatID, "\t", satelliteID[currSatID].TotalBytes, "\t", satelliteID[currSatID].TotalTransactions, "\t", satelliteID[currSatID].PutActionCount, "\t", satelliteID[currSatID].GetActionCount, "\t")
	}

	err = w.Flush()
	return err
}

func main() {
	runCmd.Flags().String("config",
		filepath.Join(defaultConfDir, "config.yaml"), "path to configuration")
	process.Exec(rootCmd)
}
