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

	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/private/version"
	"storj.io/storj/pkg/revocation"
	_ "storj.io/storj/private/version" // This attaches version information during release builds.
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/storagenodedb"
)

// StorageNodeFlags defines storage node configuration
type StorageNodeFlags struct {
	EditConf bool `default:"false" help:"open config in default editor"`

	storagenode.Config

	Deprecated
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
		Use:         "diag",
		Short:       "Diagnostic Tool support",
		RunE:        cmdDiag,
		Annotations: map[string]string{"type": "helper"},
	}
	dashboardCmd = &cobra.Command{
		Use:         "dashboard",
		Short:       "Display a dashboard",
		RunE:        cmdDashboard,
		Annotations: map[string]string{"type": "helper"},
	}
	gracefulExitInitCmd = &cobra.Command{
		Use:         "exit-satellite",
		Short:       "Initiate graceful exit",
		RunE:        cmdGracefulExitInit,
		Annotations: map[string]string{"type": "helper"},
	}
	gracefulExitStatusCmd = &cobra.Command{
		Use:         "exit-status",
		Short:       "Display graceful exit status",
		RunE:        cmdGracefulExitStatus,
		Annotations: map[string]string{"type": "helper"},
	}

	runCfg       StorageNodeFlags
	setupCfg     StorageNodeFlags
	diagCfg      storagenode.Config
	dashboardCfg struct {
		Address string `default:"127.0.0.1:7778" help:"address for dashboard service"`
	}
	defaultDiagDir string
	confDir        string
	identityDir    string
	useColor       bool
)

const (
	defaultServerAddr        = ":28967"
	defaultPrivateServerAddr = "127.0.0.1:7778"
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "storagenode")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "storagenode")
	defaultDiagDir = filepath.Join(defaultConfDir, "storage")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for storagenode configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for storagenode identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	rootCmd.PersistentFlags().BoolVar(&useColor, "color", false, "use color in user interface")
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(diagCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(gracefulExitInitCmd)
	rootCmd.AddCommand(gracefulExitStatusCmd)
	process.Bind(runCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(setupCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(configCmd, &setupCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir), cfgstruct.SetupMode())
	process.Bind(diagCmd, &diagCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(dashboardCmd, &dashboardCfg, defaults, cfgstruct.ConfDir(defaultDiagDir))
	process.Bind(gracefulExitInitCmd, &diagCfg, defaults, cfgstruct.ConfDir(defaultDiagDir))
	process.Bind(gracefulExitStatusCmd, &diagCfg, defaults, cfgstruct.ConfDir(defaultDiagDir))
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	// inert constructors only ====

	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	runCfg.Debug.Address = *process.DebugAddrFlag

	mapDeprecatedConfigs(log)

	identity, err := runCfg.Identity.Load()
	if err != nil {
		log.Fatal("Failed to load identity.", zap.Error(err))
	}

	if err := runCfg.Verify(log); err != nil {
		log.Error("Invalid configuration.", zap.Error(err))
		return err
	}

	db, err := storagenodedb.New(log.Named("db"), runCfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storagenode: %+v", err)
	}

	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	revocationDB, err := revocation.NewDBFromCfg(runCfg.Server.Config)
	if err != nil {
		return errs.New("Error creating revocation database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, revocationDB.Close())
	}()

	peer, err := storagenode.New(log, identity, db, revocationDB, runCfg.Config, version.Build)
	if err != nil {
		return err
	}

	// okay, start doing stuff ====

	_, err = peer.Version.Service.CheckVersion(ctx)
	if err != nil {
		return err
	}

	if err := process.InitMetricsWithCertPath(ctx, log, nil, runCfg.Identity.CertPath); err != nil {
		log.Warn("Failed to initialize telemetry batcher.", zap.Error(err))
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return errs.New("Error creating tables for master database on storagenode: %+v", err)
	}

	preflightEnabled, err := cmd.Flags().GetBool("preflight.database-check")
	if err != nil {
		return errs.New("Cannot retrieve preflight.database-check flag: %+v", err)
	}
	if preflightEnabled {
		err = db.Preflight(ctx)
		if err != nil {
			return errs.New("Error during preflight check for storagenode databases: %+v", err)
		}
	}

	if err := peer.Storage2.CacheService.Init(ctx); err != nil {
		log.Error("Failed to initialize CacheService.", zap.Error(err))
	}

	runError := peer.Run(ctx)
	closeError := peer.Close()

	return errs.Combine(runError, closeError)
}

func cmdSetup(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	valid, _ := fpath.IsValidSetupDir(setupDir)
	if !valid {
		return fmt.Errorf("storagenode configuration already exists (%v)", setupDir)
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	overrides := map[string]interface{}{
		"log.level": "info",
	}
	serverAddress := cmd.Flag("server.address")
	if !serverAddress.Changed {
		overrides[serverAddress.Name] = defaultServerAddr
	}

	serverPrivateAddress := cmd.Flag("server.private-address")
	if !serverPrivateAddress.Changed {
		overrides[serverPrivateAddress.Name] = defaultPrivateServerAddr
	}

	configFile := filepath.Join(setupDir, "config.yaml")
	err = process.SaveConfig(cmd, configFile, process.SaveConfigWithOverrides(overrides))
	if err != nil {
		return err
	}

	if setupCfg.EditConf {
		return fpath.EditFile(configFile)
	}

	return err
}

func cmdConfig(cmd *cobra.Command, args []string) (err error) {
	setupDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}
	//run setup if we can't access the config file
	conf := filepath.Join(setupDir, "config.yaml")
	if _, err := os.Stat(conf); err != nil {
		return cmdSetup(cmd, args)
	}

	return fpath.EditFile(conf)
}

func cmdDiag(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	diagDir, err := filepath.Abs(confDir)
	if err != nil {
		return err
	}

	// check if the directory exists
	_, err = os.Stat(diagDir)
	if err != nil {
		fmt.Println("storage node directory doesn't exist", diagDir)
		return err
	}

	db, err := storagenodedb.New(zap.L().Named("db"), diagCfg.DatabaseConfig())
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

	fmt.Fprint(w, "Satellite\tTotal\tPut\tGet\tDelete\tAudit Get\tRepair Get\tRepair Put\n")

	for _, id := range satellites {
		summary := summaries[id]
		fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
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

func main() {
	process.ExecCustomDebug(rootCmd)
}
