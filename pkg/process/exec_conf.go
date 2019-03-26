// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

// ExecuteWithConfig runs a Cobra command with the provided default config
func ExecuteWithConfig(cmd *cobra.Command, defaultConfig string) {
	flag.String("config", os.ExpandEnv(defaultConfig), "config file")
	Exec(cmd)
}

// Exec runs a Cobra command. If a "config" flag is defined it will be parsed
// and loaded using viper.
func Exec(cmd *cobra.Command) {
	exe, err := os.Executable()
	if err == nil {
		cmd.Use = exe
	}

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	cleanup(cmd)
	_ = cmd.Execute()
}

var (
	mon = monkit.Package()

	contextMtx sync.Mutex
	contexts   = map[*cobra.Command]context.Context{}
)

// SaveConfig will save only the user-specific flags with default values to
// outfile with specific values specified in 'overrides' overridden.
func SaveConfig(flagset *pflag.FlagSet, outfile string, overrides map[string]interface{}) error {
	return saveConfig(flagset, outfile, overrides, false)
}

// SaveConfigWithAllDefaults will save all flags with default values to outfile
// with specific values specified in 'overrides' overridden.
func SaveConfigWithAllDefaults(flagset *pflag.FlagSet, outfile string, overrides map[string]interface{}) error {
	return saveConfig(flagset, outfile, overrides, true)
}

func saveConfig(flagset *pflag.FlagSet, outfile string, overrides map[string]interface{}, saveAllDefaults bool) error {
	// we previously used Viper here, but switched to a custom serializer to allow comments
	//todo:  switch back to Viper once go-yaml v3 is released and its supports writing comments?
	flagset.AddFlagSet(pflag.CommandLine)
	//sort keys
	var keys []string
	flagset.VisitAll(func(f *pflag.Flag) { keys = append(keys, f.Name) })
	sort.Strings(keys)
	//serialize
	var sb strings.Builder
	w := &sb
	for _, k := range keys {
		f := flagset.Lookup(k)
		if readBoolAnnotation(f, "setup") {
			continue
		}

		var overriddenValue interface{}
		var overrideExist bool
		if overrides != nil {
			overriddenValue, overrideExist = overrides[k]
		}

		if !saveAllDefaults && !readBoolAnnotation(f, "user") && !f.Changed && !overrideExist {
			continue
		}

		value := f.Value.String()
		if overriddenValue != nil {
			value = fmt.Sprintf("%v", overriddenValue)
		}
		if f.Usage != "" {
			fmt.Fprintf(w, "# %s\n", f.Usage)
		}
		fmt.Fprintf(w, "%s: ", k)
		switch f.Value.Type() {
		case "string":
			// save ourselves 250+ lines of code and just double quote strings
			fmt.Fprintf(w, "%q\n", value)
		default:
			//assume that everything else doesn't have fancy control characters
			fmt.Fprintf(w, "%s\n", value)
		}
	}

	err := ioutil.WriteFile(outfile, []byte(sb.String()), os.FileMode(0644))
	if err != nil {
		return err
	}
	fmt.Println("Configuration saved to:", outfile)
	return nil
}

func readBoolAnnotation(flag *pflag.Flag, key string) bool {
	annotation := flag.Annotations[key]
	return len(annotation) > 0 && annotation[0] == "true"
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

		cfgFlag := cmd.Flags().Lookup("config-dir")
		if cfgFlag != nil && cfgFlag.Value.String() != "" {
			path := filepath.Join(os.ExpandEnv(cfgFlag.Value.String()), "config.yaml")
			if cmd.Annotations["type"] != "setup" || fileExists(path) {
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
				oldChanged := cmd.Flag(key).Changed
				err := cmd.Flags().Set(key, vip.GetString(key))
				if err != nil {
					// flag couldn't be set
					brokenVals = append(brokenVals, key)
				}
				// revert Changed value
				cmd.Flag(key).Changed = oldChanged
			}
		}

		logger, err := newLogger()
		if err != nil {
			return err
		}

		if vip.ConfigFileUsed() != "" {
			path, err := filepath.Abs(vip.ConfigFileUsed())
			if err != nil {
				path = vip.ConfigFileUsed()
				logger.Debug("unable to resolve path", zap.Error(err))
			}

			logger.Sugar().Info("Configuration loaded from: ", path)
		}

		defer func() { _ = logger.Sync() }()
		defer zap.ReplaceGlobals(logger)()
		defer zap.RedirectStdLog(logger)()

		// okay now that logging is working, inform about the broken keys
		if cmd.Annotations["type"] != "helper" {
			for _, key := range brokenKeys {
				logger.Sugar().Infof("Invalid configuration file key: %s", key)
			}
		}
		for _, key := range brokenVals {
			logger.Sugar().Infof("Invalid configuration file value for key: %s", key)
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
			_, _ = fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
			logger.Sugar().Debugf("Fatal error: %+v", err)
			_ = logger.Sync()
			os.Exit(1)
		}
		return err
	}
}
