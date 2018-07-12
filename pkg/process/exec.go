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

const configType = ".json"

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
func readFlags() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
}

// get config path
func configPath() string {
	home, _ := homedir.Dir()
	return filepath.Join(home, ".storj")
}

func generateConfig() error {
	readFlags()
	storj := filepath.Join(configPath(), "main.json")
	err := viper.WriteConfigAs(storj)
	return err
}

// ConfigEnv will read in command line flags, set the name of the config file,
// then look for configs in the current working directory and in $HOME/.storj
func ConfigEnv() (*viper.Viper, error) {
	viper.SetEnvPrefix("storj")
	viper.AutomaticEnv()
	viper.SetConfigName("main")
	viper.AddConfigPath(".")
	viper.AddConfigPath(configPath())

	err := viper.ReadInConfig()
	if err != nil {
		log.Print("cannot find config file, generating new config from defaults.")
		if err := generateConfig(); err != nil {
			log.Print("error generating config", err)
		}
	}

	readFlags()
	v := viper.GetViper()
	return v, nil
}

// Execute runs a *cobra.Command and sets up Storj-wide process configuration
// like a configuration file and logging.
func Execute(cmd *cobra.Command) {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	cobra.OnInitialize(func() {
		ConfigEnv()
		viper.BindPFlags(cmd.Flags())
		viper.SetEnvPrefix("storj")
		viper.AutomaticEnv()
	})

	Must(cmd.Execute())
}

// Main runs a Service
func Main(configFn func() (*viper.Viper, error), s ...Service) error {
	configFn()
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
