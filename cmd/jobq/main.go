// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/peertls"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/pkcrypto"
	"storj.io/common/process"
	"storj.io/storj/private/revocation"
	"storj.io/storj/satellite/jobq"
	"storj.io/storj/satellite/jobq/server"
)

// Config is the configuration for the job queue server.
type Config struct {
	Identity      identity.Config
	ListenAddress string `help:"address to listen on" default:":15781" testDefault:"$HOST:0"`
	// InitAlloc is the initial allocation size for the job queue, in bytes.
	// There is no special need to keep this low; unused parts of the queue
	// allocation will not take up system memory until the queue grows to that
	// size.
	InitAlloc memory.Size `help:"initial allocation size for the job queue, in bytes" default:"2GiB"`
	// MemReleaseThreshold is the memory release threshold for the job queue, in
	// bytes. When the job queue has more than this amount of memory mapped to
	// empty pages (because the queue shrunk considerably), the unused memory
	// will be marked as unused (if supported) and the OS will be allowed to
	// reclaim it.
	MemReleaseThreshold memory.Size `help:"element memory release threshold for the job queue, in bytes" default:"100MiB"`
	// RetryAfter is the time to wait before retrying a failed job. If jobs are
	// pushed to the queue with a LastAttemptedAt more recent than this duration
	// ago, they will go into the retry queue instead of the repair queue, until
	// they are eligible to go in the repair queue.
	RetryAfter time.Duration `help:"time to wait before retrying a failed job" default:"1h"`
	// TLS is the configuration for the server's TLS.
	TLS tlsopts.Config
}

var (
	confDir     string
	identityDir string

	runCfg Config

	rootCmd = &cobra.Command{
		Use:   "jobq",
		Short: "job queue server (implements the repair queue)",
		RunE:  runJobQueue,
	}
)

func init() {
	defaultConfDir := fpath.ApplicationDir("storj", "jobq")
	defaultIdentityDir := fpath.ApplicationDir("storj", "identity", "jobq")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &confDir, "config-dir", defaultConfDir, "main directory for jobq configuration")
	cfgstruct.SetupFlag(zap.L(), rootCmd, &identityDir, "identity-dir", defaultIdentityDir, "main directory for jobq identity credentials")
	defaults := cfgstruct.DefaultsFlag(rootCmd)
	process.Bind(rootCmd, &runCfg, defaults, cfgstruct.ConfDir(confDir), cfgstruct.IdentityDir(identityDir))
}

func runJobQueue(cmd *cobra.Command, args []string) error {
	logger := zap.L()
	ctx := context.Background()

	addr, err := net.ResolveTCPAddr("tcp", runCfg.ListenAddress)
	if err != nil {
		return fmt.Errorf("failed to resolve listen address %q: %w", runCfg.ListenAddress, err)
	}
	identity, err := runCfg.Identity.Load()
	if err != nil {
		return fmt.Errorf("failed to load identity: %w", err)
	}
	revocationDB, err := revocation.OpenDBFromCfg(ctx, runCfg.TLS)
	if err != nil {
		return fmt.Errorf("error creating revocation database: %w", err)
	}
	tlsOpts, err := tlsopts.NewOptions(identity, runCfg.TLS, revocationDB)
	if err != nil {
		return fmt.Errorf("failed to create TLS options: %w", err)
	}
	// the tlsopts machinery does not apply the peer CA whitelist to the server
	// side configuration, so we do it here.
	err = applyPeerCAWhitelist(runCfg.TLS.UsePeerCAWhitelist, runCfg.TLS.PeerCAWhitelistPath, tlsOpts)
	if err != nil {
		return fmt.Errorf("failed to apply peer CA whitelist: %w", err)
	}
	initElements := uint64(runCfg.InitAlloc) / uint64(jobq.RecordSize)
	memReleaseThrehsold := uint64(runCfg.MemReleaseThreshold) / uint64(jobq.RecordSize)
	logger.Debug("initializing job queue", zap.Uint64("elements before queue resize", initElements), zap.Uint64("element mem release threshold", memReleaseThrehsold))
	srv, err := server.New(logger, addr, tlsOpts, runCfg.RetryAfter, int(initElements), int(memReleaseThrehsold))
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	err = srv.Run(ctx)
	if err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

func main() {
	logger, _, _ := process.NewLogger("jobq")
	zap.ReplaceGlobals(logger)

	process.Exec(rootCmd)
}

func applyPeerCAWhitelist(usePeerCAWhitelist bool, peerCAWhitelistPath string, tlsOpts *tlsopts.Options) (err error) {
	if usePeerCAWhitelist {
		whitelist := []byte(tlsopts.DefaultPeerCAWhitelist)
		if peerCAWhitelistPath != "" {
			whitelist, err = os.ReadFile(peerCAWhitelistPath)
			if err != nil {
				return fmt.Errorf("unable to find whitelist file %v: %w", peerCAWhitelistPath, err)
			}
		}
		tlsOpts.PeerCAWhitelist, err = pkcrypto.CertsFromPEM(whitelist)
		if err != nil {
			return err
		}
		tlsOpts.VerificationFuncs.ServerAdd(peertls.VerifyCAWhitelist(tlsOpts.PeerCAWhitelist))
	}
	return nil
}
