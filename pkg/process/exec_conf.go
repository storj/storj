// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/telemetry"
	"storj.io/storj/pkg/utils"
)

// ExecuteWithConfig runs a Cobra command with the provided default config
func ExecuteWithConfig(cmd *cobra.Command, defaultConfig string) {
	cfgFile := flag.String("config", os.ExpandEnv(defaultConfig), "config file")
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	cleanup(cmd, cfgFile)
	_ = cmd.Execute()
}

var (
	mon = monkit.Package()

	contextMtx sync.Mutex
	contexts   = map[*cobra.Command]context.Context{}
)

// Ctx returns the appropriate context.Context for ExecuteWithConfig commands
func Ctx(cmd *cobra.Command) context.Context {
	contextMtx.Lock()
	defer contextMtx.Unlock()
	ctx := contexts[cmd]
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type ctxKey int

const (
	ctxKeyVip ctxKey = iota
	ctxKeyCfg
)

// SaveConfig outputs the configuration to the configured (or default) config
// file given to ExecuteWithConfig
func SaveConfig(cmd *cobra.Command) error {
	ctx := Ctx(cmd)
	return getViper(ctx).WriteConfigAs(CfgPath(ctx))
}

// SaveConfigAs outputs the configuration to the provided path assuming the
// command was executed with ExecuteWithConfig
func SaveConfigAs(cmd *cobra.Command, path string) error {
	return getViper(Ctx(cmd)).WriteConfigAs(path)
}

func getViper(ctx context.Context) *viper.Viper {
	if v, ok := ctx.Value(ctxKeyVip).(*viper.Viper); ok {
		return v
	}
	return nil
}

// CfgPath returns the configuration path used with ExecuteWithConfig
func CfgPath(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyCfg).(string); ok {
		return v
	}
	return ""
}

func cleanup(cmd *cobra.Command, cfgFile *string) {
	for _, ccmd := range cmd.Commands() {
		cleanup(ccmd, cfgFile)
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
		vip.AutomaticEnv()
		if *cfgFile != "" && fileExists(*cfgFile) {
			vip.SetConfigFile(*cfgFile)
			err = vip.ReadInConfig()
			if err != nil {
				return err
			}
		}

		// go back and propagate changed config values to appropriate flags
		var brokenKeys []string
		for _, key := range vip.AllKeys() {
			if cmd.Flags().Lookup(key) == nil {
				// flag couldn't be found
				brokenKeys = append(brokenKeys, key)
			} else {
				err := cmd.Flags().Set(key, vip.GetString(key))
				if err != nil {
					// flag couldn't be set
					brokenKeys = append(brokenKeys, key)
				}
			}
		}

		ctx = context.WithValue(ctx, ctxKeyVip, vip)
		ctx = context.WithValue(ctx, ctxKeyCfg, *cfgFile)

		logger, err := utils.NewLogger(*logDisposition)
		if err != nil {
			return err
		}
		defer func() { _ = logger.Sync() }()
		defer zap.ReplaceGlobals(logger)()
		defer zap.RedirectStdLog(logger)()

		// okay now that logging is working, inform about the broken keys
		// these keys are almost certainly broken because they have capital
		// letters
		if len(brokenKeys) > 0 {
			logger.Sugar().Infof("TODO: these flags are not configurable via "+
				"config file, probably due to having uppercase letters: %s",
				strings.Join(brokenKeys, ", "))
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
			log.Fatalf("%+v", err)
		}
		return err
	}
}
