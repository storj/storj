// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func defaultConfigPath(name string) string {
	if name == "" {
		name = filepath.Base(os.Args[0])
	}
	path := filepath.Join(".storj", fmt.Sprintf("%s.json", name))
	home, err := homedir.Dir()
	if err != nil {
		log.Println(err)
		return path
	}
	return filepath.Join(home, path)
}

// ReadFlags will read in and bind flags for viper and pflag
func ReadFlags() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
}

// get config path
func configPath() string {
	home, _ := homedir.Dir()
	return filepath.Join(home, ".storj")
}

// AddConfig will add a config to viper
func AddConfig() error {
	home, err := homedir.Dir()
	storj := filepath.Join(home, ".storj")
	viper.SetConfigName("main")
	viper.AddConfigPath(".")
	viper.AddConfigPath(storj)
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatal("error reading in config", err)
		return err
	}
	return nil
}

func writeConfig(v *viper.Viper) error {
	return v.WriteConfig()
}

// Execute runs a *cobra.Command and sets up Storj-wide process configuration
// like a configuration file and logging.
func Execute(cmd *cobra.Command) {
	cfgFile := flag.String("config", defaultConfigPath(cmd.Name()),
		"config file")

	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	cobra.OnInitialize(func() {
		viper.BindPFlags(cmd.Flags())
		viper.SetEnvPrefix("storj")
		viper.AutomaticEnv()
		if *cfgFile != "" {
			viper.SetConfigFile(*cfgFile)
			viper.ReadInConfig()
		}
	})

	Must(cmd.Execute())
}

// ConfigEnvironment sets up a standard Viper environment and parses CLI flags
func ConfigEnvironment() (*viper.Viper, error) {
	viper.SetEnvPrefix("storj")
	viper.AutomaticEnv()
	return nil, nil
}

// Main runs a Service
func Main(configFn func() (*viper.Viper, error), s ...Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	errors := make(chan error, len(s))

	for _, service := range s {
		go func(ctx context.Context, s Service, ch <-chan error) {
			errors <- CtxService(s)(&cobra.Command{}, pflag.Args())
		}(ctx, service, errors)
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-errors:
		cancel()
		return err
	}
}

// Must checks for errors
func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
