// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package internalcmd

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/private/process"
	"storj.io/storj/storagenode/pieces/lazyfilewalker"
	"storj.io/storj/storagenode/storagenodedb"
)

// FilewalkerCfg is the config structure for the lazyfilewalker commands.
type FilewalkerCfg struct {
	lazyfilewalker.Config
}

// RunOptions defines the options for the lazyfilewalker runners.
type RunOptions struct {
	Ctx    context.Context
	Logger *zap.Logger
	Config *FilewalkerCfg

	stdin  io.Reader
	stderr io.Writer
	stdout io.Writer
}

type nopWriterSyncCloser struct {
	io.Writer
}

func (cw nopWriterSyncCloser) Close() error { return nil }
func (cw nopWriterSyncCloser) Sync() error  { return nil }

// SetOut sets the stdout writer.
func (r *RunOptions) SetOut(writer io.Writer) {
	r.stdout = writer
}

// SetErr sets the stderr writer.
func (r *RunOptions) SetErr(writer io.Writer) {
	r.stderr = writer
	writerkey := "zapwriter"

	// If the writer is os.Stderr, we don't need to register it because the stderr
	// writer is registered by default.
	if writer == os.Stderr {
		return
	}

	err := zap.RegisterSink(writerkey, func(u *url.URL) (zap.Sink, error) {
		return nopWriterSyncCloser{r.stderr}, nil
	})

	// this error is expected if the sink is already registered.
	duplicateSinkErr := fmt.Errorf("sink factory already registered for scheme %q", writerkey)
	if err != nil && err.Error() != duplicateSinkErr.Error() {
		r.Logger.Error("failed to register logger sink", zap.Error(err))
		return
	}

	err = flag.Set("log.encoding", "json")
	if err != nil {
		r.Logger.Error("failed to set log encoding", zap.Error(err))
		return
	}

	// create a new logger with the writer as the output path.
	path := fmt.Sprintf("%s:subprocess", writerkey)
	logger, err := process.NewLoggerWithOutputPaths("lazyfilewalker", path)
	if err != nil {
		r.Logger.Error("failed to create logger", zap.Error(err))
		return
	}

	// set the logger to the new logger.
	r.Logger = logger
}

// SetIn sets the stdin reader.
func (r *RunOptions) SetIn(reader io.Reader) {
	r.stdin = reader
}

// DefaultRunOpts returns the default RunOptions.
func DefaultRunOpts(ctx context.Context, logger *zap.Logger, config *FilewalkerCfg) *RunOptions {
	return &RunOptions{
		Ctx:    ctx,
		Logger: logger,
		Config: config,
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// DatabaseConfig returns the storagenodedb.Config that should be used with this lazyfilewalker.
func (config *FilewalkerCfg) DatabaseConfig() storagenodedb.Config {
	return storagenodedb.Config{
		Storage:   config.Storage,
		Info:      config.Info,
		Info2:     config.Info2,
		Pieces:    config.Pieces,
		Filestore: config.Filestore,
		Driver:    config.Driver,
	}
}

// NewUsedSpaceFilewalkerCmd creates a new cobra command for running used-space calculation filewalker.
func NewUsedSpaceFilewalkerCmd() *cobra.Command {
	var cfg FilewalkerCfg

	cmd := &cobra.Command{
		Use:   lazyfilewalker.UsedSpaceFilewalkerCmdName,
		Short: "An internal subcommand used to run used-space calculation filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			return NewUsedSpaceLazyFilewalkerWithConfig(ctx, zap.L(), &cfg).Run()
		},
		Hidden: true,
		Args:   cobra.ExactArgs(0),
	}

	process.Bind(cmd, &cfg)

	return cmd
}

// NewGCFilewalkerCmd creates a new cobra command for running garbage collection filewalker.
func NewGCFilewalkerCmd() *cobra.Command {
	var cfg FilewalkerCfg

	cmd := &cobra.Command{
		Use:   lazyfilewalker.GCFilewalkerCmdName,
		Short: "An internal subcommand used to run garbage collection filewalker as a separate subprocess with lower IO priority",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, _ := process.Ctx(cmd)
			return NewGCLazyFilewalkerWithConfig(ctx, zap.L(), &cfg).Run()
		},
		Hidden: true,
		Args:   cobra.ExactArgs(0),
	}

	process.Bind(cmd, &cfg)

	return cmd
}
