// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"github.com/zeebo/structs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/collect"
	"gopkg.in/spacemonkeygo/monkit.v2/present"

	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/cfgstruct"
)

const (
	// DefaultCfgFilename is the default filename used for storing a configuration.
	DefaultCfgFilename       = "config.yaml"
	DefaultSecureCfgFilename = ".secure-config.yaml"
)

var (
	mon = monkit.Package()

	contextMtx sync.Mutex
	contexts   = map[*cobra.Command]context.Context{}

	configMtx sync.Mutex
	configs   = map[*cobra.Command][]interface{}{}
	vipers    = map[*cobra.Command]*viperState{}
)

type viperState struct {
	vip     *viper.Viper
	configs []string
}

// Bind sets flags on a command that match the configuration struct
// 'config'. It ensures that the config has all of the values loaded into it
// when the command runs.
func Bind(cmd *cobra.Command, config interface{}, opts ...cfgstruct.BindOpt) {
	configMtx.Lock()
	defer configMtx.Unlock()

	cfgstruct.Bind(cmd.Flags(), config, opts...)
	configs[cmd] = append(configs[cmd], config)
}

// Exec runs a Cobra command. If a "config" flag is defined it will be parsed
// and loaded using viper.
func Exec(cmd *cobra.Command) {
	cmd.AddCommand(&cobra.Command{
		Use:         "version",
		Short:       "output the version's build information, if any",
		RunE:        cmdVersion,
		Annotations: map[string]string{"type": "setup"}})

	exe, err := os.Executable()
	if err == nil {
		cmd.Use = exe
	}

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	cleanup(cmd)
	_ = cmd.Execute()
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
		sig := <-c
		log.Printf("Got a signal from the OS: %q", sig)
		signal.Stop(c)
		cancel()
	}()
	return ctx
}

// getViperState creates and caches or returns the cached viper as well as the
// set of configuration files loaded into it.
func getViperState(cmd *cobra.Command) (*viper.Viper, []string, error) {
	configMtx.Lock()
	defer configMtx.Unlock()

	if state, ok := vipers[cmd]; ok {
		return state.vip, state.configs, nil
	}

	vip := viper.New()
	err := vip.BindPFlags(cmd.Flags())
	if err != nil {
		return nil, nil, err
	}
	vip.SetEnvPrefix("storj")
	vip.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	vip.AutomaticEnv()

	var configFiles []string
	mergeConfig := func(flagName, fileName string, required bool) error {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil || flag.Value.String() == "" {
			return nil
		}
		path := filepath.Join(os.ExpandEnv(flag.Value.String()), fileName)
		if !required && !fileExists(path) {
			return nil
		}
		vip.SetConfigFile(path)
		configFiles = append(configFiles, path)
		return vip.MergeInConfig()
	}

	// The default config file is required if the command is not a setup command.
	configRequired := cmd.Annotations["type"] != "setup"
	if err := mergeConfig("config-dir", DefaultCfgFilename, configRequired); err != nil {
		return nil, nil, err
	}
	// The secure config file is never required.
	if err := mergeConfig("config-dir", DefaultSecureCfgFilename, false); err != nil {
		return nil, nil, err
	}

	vipers[cmd] = &viperState{vip: vip, configs: configFiles}
	return vip, configFiles, nil
}

var traceOut = flag.String("debug.trace-out", "", "If set, a path to write a process trace SVG to")

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

		// Load/cache the viper state for the command
		vip, configFiles, err := getViperState(cmd)
		if err != nil {
			return err
		}

		// Load the viper into the configs bound to the command.
		configMtx.Lock()
		configValues := configs[cmd]
		configMtx.Unlock()

		var (
			brokenKeys  = map[string]struct{}{}
			missingKeys = map[string]struct{}{}
			usedKeys    = map[string]struct{}{}
			allSettings = vip.AllSettings()
		)

		// Hacky hack: these two keys are noprefix which breaks all scoping
		if val, ok := allSettings["api-key"]; ok {
			allSettings["client.api-key"] = val
			delete(allSettings, "api-key")
		}
		if val, ok := allSettings["satellite-addr"]; ok {
			allSettings["client.satellite-addr"] = val
			delete(allSettings, "satellite-addr")
		}

		// Decode and all of the resulting keys into our sets
		for _, config := range configValues {
			res := structs.Decode(allSettings, config)
			for key := range res.Used {
				usedKeys[key] = struct{}{}
			}
			for key := range res.Missing {
				missingKeys[key] = struct{}{}
			}
			for key := range res.Broken {
				brokenKeys[key] = struct{}{}
			}
		}

		// Filter the missing keys by removing ones that were used and ones that are flags.
		for key := range usedKeys {
			delete(missingKeys, key)
		}
		for key := range missingKeys {
			if cmd.Flags().Lookup(key) != nil {
				delete(missingKeys, key)
			}
		}

		// Set up logging.
		logger, err := newLogger()
		if err != nil {
			return err
		}
		defer func() { _ = logger.Sync() }()
		defer zap.ReplaceGlobals(logger)()
		defer zap.RedirectStdLog(logger)()

		// Log about which configuration files we used.
		for _, path := range configFiles {
			absPath, err := filepath.Abs(path)
			if err != nil {
				absPath = path
				logger.Debug("unable to resolve path", zap.Error(err))
			}
			logger.Sugar().Info("Configuration loaded from: ", absPath)
		}

		// Log about the configuration values that had problems.
		if cmd.Annotations["type"] != "helper" {
			for key := range missingKeys {
				logger.Sugar().Infof("Invalid configuration file key: %s", key)
			}
		}
		for key := range brokenKeys {
			logger.Sugar().Infof("Invalid configuration file value for key: %s", key)
		}

		// Init debugging.
		err = initDebug(logger, monkit.Default)
		if err != nil {
			logger.Error("failed to start debug endpoints", zap.Error(err))
		}

		// Set up the work closure.
		var workErr error
		work := func(ctx context.Context) {
			contextMtx.Lock()
			contexts[cmd] = ctx
			contextMtx.Unlock()
			defer func() {
				contextMtx.Lock()
				delete(contexts, cmd)
				contextMtx.Unlock()
			}()

			workErr = internalRun(cmd, args)
		}

		// Dispatch to work, rendering an svg if required.
		if *traceOut != "" {
			fh, err := os.Create(*traceOut)
			if err != nil {
				return err
			}
			err = present.SpansToSVG(fh, collect.CollectSpans(ctx, work))
			err = errs.Combine(err, fh.Close())
			if err != nil {
				logger.Error("failed to write svg", zap.Error(err))
			}
		} else {
			work(ctx)
		}

		// Log/return any errors from the work
		err = workErr
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
			logger.Sugar().Debugf("Fatal error: %+v", err)
			_ = logger.Sync()
			os.Exit(1)
		}
		return err
	}
}

func cmdVersion(cmd *cobra.Command, args []string) (err error) {
	if version.Build.Release {
		fmt.Println("Release build")
	} else {
		fmt.Println("Development build")
	}

	if version.Build.Version != (version.SemVer{}) {
		fmt.Println("Version:", version.Build.Version.String())
	}
	if !version.Build.Timestamp.IsZero() {
		fmt.Println("Build timestamp:", version.Build.Timestamp.Format(time.RFC822))
	}
	if version.Build.CommitHash != "" {
		fmt.Println("Git commit:", version.Build.CommitHash)
	}
	return err
}
