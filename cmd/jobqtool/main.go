// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/storj"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite/jobq"
)

// Config holds the toplevel configuration for jobqtool.
type Config struct {
	Identity identity.Config
	Server   string `help:"address of the job queue server" default:"localhost:15781" testDefault:"$HOST:0"`
	TLS      tlsopts.Config
}

var (
	confDir     string
	identityDir string

	runCfg Config

	rootCmd = &cobra.Command{
		Use:          "jobqtool",
		Short:        "job queue tool",
		SilenceUsage: true,
	}
	lenCmd = &cobra.Command{
		Use:   "len [<placement>]",
		Short: "query lengths of job queues (repair and retry) for the given placement",
		RunE:  lenCommand,
		Args:  cobra.MaximumNArgs(1),
	}
	truncateCmd = &cobra.Command{
		Use:   "truncate <placement>",
		Short: "empty job queues (repair and retry) for the given placement",
		RunE:  truncateCommand,
		Args:  cobra.ExactArgs(1),
	}
	statCmd = &cobra.Command{
		Use:   "stat [<placement>]",
		Short: "query statistics of job queues (repair and retry) for the given placement",
		RunE:  statCommand,
		Args:  cobra.MaximumNArgs(1),
	}
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "jobqtool")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "jobqtool")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for jobqtool configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for jobqtool identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	process.Bind(rootCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(lenCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(truncateCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(statCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	rootCmd.AddCommand(lenCmd)
	rootCmd.AddCommand(truncateCmd)
	rootCmd.AddCommand(statCmd)
}

func prepareConnection(ctx context.Context) (*jobq.Client, error) {
	identity, err := runCfg.Identity.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load identity: %w", err)
	}
	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.TLS)
	if err != nil {
		return nil, fmt.Errorf("creating revocation database: %w", err)
	}
	tlsOpts, err := tlsopts.NewOptions(identity, runCfg.TLS, revocationDB)
	if err != nil {
		return nil, fmt.Errorf("TLS options: %w", err)
	}

	conn, err := tls.Dial("tcp", runCfg.Server, tlsOpts.UnverifiedClientTLSConfig())
	if err != nil {
		return nil, err
	}

	return jobq.WrapConn(conn), nil
}

func lenCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	var repairLen, retryLen int64
	if len(args) > 0 {
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		repairLen, retryLen, err = drpcConn.Len(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("querying queue length: %w", err)
		}
	} else {
		repairLen, retryLen, err = drpcConn.LenAll(ctx)
		if err != nil {
			return fmt.Errorf("querying queue lengths: %w", err)
		}
	}
	fmt.Printf("repair %d\nretry %d\n", repairLen, retryLen)
	return nil
}

func truncateCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	placement, err := strconv.ParseInt(args[0], 10, 16)
	if err != nil {
		return fmt.Errorf("invalid placement %q: %w", args[0], err)
	}
	err = drpcConn.Truncate(ctx, storj.PlacementConstraint(placement))
	if err != nil {
		return fmt.Errorf("failed to truncate queue: %w", err)
	}
	return nil
}

func statCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	drpcConn, err := prepareConnection(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = drpcConn.Close() }()

	var jobStats []jobq.QueueStat
	if len(args) > 0 {
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		jobStat, err := drpcConn.Stat(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("querying queue statistics: %w", err)
		}
		jobStats = []jobq.QueueStat{jobStat}
	} else {
		jobStats, err = drpcConn.StatAll(ctx)
		if err != nil {
			return fmt.Errorf("querying all queue statistics: %w", err)
		}
	}
	outputJobs := false
	for _, stat := range jobStats {
		if stat.Count == 0 {
			continue
		}
		outputJobs = true
		if stat.MinAttemptedAt == nil {
			fmt.Printf("placement %d [repair]\n", stat.Placement)
		} else {
			fmt.Printf("placement %d [waiting for retry]\n", stat.Placement)
		}
		fmt.Printf("  count %d\n", stat.Count)
		fmt.Printf("  min inserted at %v\n", stat.MinInsertedAt)
		fmt.Printf("  max inserted at %v\n", stat.MaxInsertedAt)
		if stat.MinAttemptedAt != nil {
			fmt.Printf("  min attempted at %v\n", stat.MinAttemptedAt)
			fmt.Printf("  max attempted at %v\n", stat.MaxAttemptedAt)
		}
		fmt.Printf("  min segment health: %.6f\n", stat.MinSegmentHealth)
		fmt.Printf("  max segment health: %.6f\n", stat.MaxSegmentHealth)
	}
	if !outputJobs {
		fmt.Println("(no jobs)")
	}
	return nil
}

func main() {
	logger, atomicLevel, _ := process.NewLogger("jobqtool")
	atomicLevel.SetLevel(zap.WarnLevel)
	zap.ReplaceGlobals(logger)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}
