// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/telemetry"
)

// ExecuteWithConfig runs a Cobra command with the provided default config
func ExecuteWithConfig(cmd *cobra.Command, defaultConfig string) {
	flag.String("config", os.ExpandEnv(defaultConfig), "config file")
	Exec(cmd)
}

// Exec runs a Cobra command. If a "config" flag is defined it will be parsed
// and loaded using viper.
func Exec(cmd *cobra.Command) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	cleanup(cmd)
	_ = cmd.Execute()
}

var (
	mon = monkit.Package()

	contextMtx sync.Mutex
	contexts   = map[*cobra.Command]context.Context{}
)

// SaveConfig will save all flags with default values to outfilewith specific
// values specified in 'overrides' overridden.
func SaveConfig(flagset *pflag.FlagSet, outfile string, overrides map[string]interface{}) error {

	vip := viper.New()
	err := vip.BindPFlags(pflag.CommandLine)
	if err != nil {
		return err
	}
	flagset.VisitAll(func(f *pflag.Flag) {
		// stop processing if we hit an error on a BindPFlag call
		if err != nil {
			return
		}
		if f.Name == "config" {
			return
		}
		err = vip.BindPFlag(f.Name, f)
	})
	if err != nil {
		return err
	}

	for key, val := range overrides {
		vip.Set(key, val)
	}

	return vip.WriteConfigAs(os.ExpandEnv(outfile))
}

// Ctx returns the appropriate context.Context for ExecuteWithConfig commands
func Ctx(cmd *cobra.Command) context.Context {
	contextMtx.Lock()
	defer contextMtx.Unlock()
	ctx := contexts[cmd]
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		signal.Stop(c)
		cancel()
	}()
	return ctx
}

func cleanup(cmd *cobra.Command) {
	for _, ccmd := range cmd.Commands() {
		cleanup(ccmd)
	}
	if cmd.Run != nil {
		panic("Please use cobra's RunE instead of Run")
	}
	internalRun := cmd.RunE
	if internalRun == nil {
		return
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) (err error) {
		ctx := context.Background()
		defer mon.TaskNamed("root")(&ctx)(&err)

		vip := viper.New()
		err = vip.BindPFlags(cmd.Flags())
		if err != nil {
			return err
		}
		vip.SetEnvPrefix("storj")
		vip.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
		vip.AutomaticEnv()

		cfgFlag := cmd.Flags().Lookup("config")
		if cfgFlag != nil && cfgFlag.Value.String() != "" {
			path := os.ExpandEnv(cfgFlag.Value.String())
			if cfgFlag.Changed || fileExists(path) {
				vip.SetConfigFile(path)
				err = vip.ReadInConfig()
				if err != nil {
					return err
				}
			}
		}

		// go back and propagate changed config values to appropriate flags
		var brokenKeys []string
		var brokenVals []string
		for _, key := range vip.AllKeys() {
			if cmd.Flags().Lookup(key) == nil {
				// flag couldn't be found
				brokenKeys = append(brokenKeys, key)
			} else {
				err := cmd.Flags().Set(key, vip.GetString(key))
				if err != nil {
					// flag couldn't be set
					brokenVals = append(brokenVals, key)
				}
			}
		}

		logger, err := newLogger()
		if err != nil {
			return err
		}
		defer func() { _ = logger.Sync() }()
		defer zap.ReplaceGlobals(logger)()
		defer zap.RedirectStdLog(logger)()

		logger.Debug("logging initialized")

		// okay now that logging is working, inform about the broken keys
		for _, key := range brokenKeys {
			logger.Sugar().Infof("Invalid configuration file key: %s", key)
		}
		for _, key := range brokenVals {
			logger.Sugar().Infof("Invalid configuration file value for key: %s", key)
		}

		err = initMetrics(ctx, monkit.Default,
			telemetry.DefaultInstanceID())
		if err != nil {
			logger.Error("failed to configure telemetry", zap.Error(err))
		}

		err = initDebug(logger, monkit.Default)
		if err != nil {
			logger.Error("failed to start debug endpoints", zap.Error(err))
		}

		contextMtx.Lock()
		contexts[cmd] = ctx
		contextMtx.Unlock()
		defer func() {
			contextMtx.Lock()
			delete(contexts, cmd)
			contextMtx.Unlock()
		}()

		err = internalRun(cmd, args)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
			logger.Sugar().Debugf("%+v", err)
			_ = logger.Sync()
			os.Exit(1)
		}
		return err
	}
}
