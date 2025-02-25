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
	Server   string `help:"address of the job queue server" default:"localhost:15781"`
	TLS      tlsopts.Config
}

var (
	confDir     string
	identityDir string

	runCfg Config

	rootCmd = &cobra.Command{
		Use:   "jobqtool",
		Short: "job queue tool",
	}
	lenCmd = &cobra.Command{
		Use:   "len [placement]",
		Short: "query lengths of job queues (repair and retry) for the given placement",
		RunE:  runCommand,
		Args:  cobra.ExactArgs(1),
	}
	truncateCmd = &cobra.Command{
		Use:   "truncate [placement]",
		Short: "empty job queues (repair and retry) for the given placement",
		RunE:  runCommand,
		Args:  cobra.ExactArgs(1),
	}
	addQueueCmd = &cobra.Command{
		Use:   "add-queue [placement]",
		Short: "allocate a new queue for the given placement constraint",
		RunE:  runCommand,
		Args:  cobra.ExactArgs(1),
	}
	destroyQueueCmd = &cobra.Command{
		Use:   "destroy-queue [placement]",
		Short: "destroy the queue for the given placement constraint",
		RunE:  runCommand,
		Args:  cobra.ExactArgs(1),
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
	process.Bind(addQueueCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	process.Bind(destroyQueueCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
	rootCmd.AddCommand(lenCmd)
	rootCmd.AddCommand(truncateCmd)
	rootCmd.AddCommand(addQueueCmd)
	rootCmd.AddCommand(destroyQueueCmd)
}

func runCommand(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	identity, err := runCfg.Identity.Load()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}
	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.TLS)
	if err != nil {
		return fmt.Errorf("creating revocation database: %w", err)
	}
	tlsOpts, err := tlsopts.NewOptions(identity, runCfg.TLS, revocationDB)
	if err != nil {
		return fmt.Errorf("TLS options: %w", err)
	}

	conn, err := tls.Dial("tcp", runCfg.Server, tlsOpts.UnverifiedClientTLSConfig())
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	drpcConn := jobq.WrapConn(conn)

	switch cmd.Name() {
	case "len":
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		repairLen, retryLen, err := drpcConn.Len(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("querying queue length: %w", err)
		}
		fmt.Printf("repair %d\nretry %d\n", repairLen, retryLen)
	case "truncate":
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		err = drpcConn.Truncate(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("failed to truncate queue: %w", err)
		}
	case "add-queue":
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		err = drpcConn.AddPlacementQueue(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("failed to add queue: %w", err)
		}
	case "destroy-queue":
		placement, err := strconv.ParseInt(args[0], 10, 16)
		if err != nil {
			return fmt.Errorf("invalid placement %q: %w", args[0], err)
		}
		err = drpcConn.DestroyPlacementQueue(ctx, storj.PlacementConstraint(placement))
		if err != nil {
			return fmt.Errorf("failed to destroy queue: %w", err)
		}
	default:
		return fmt.Errorf("unrecognized command %q", cmd.Name())
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
