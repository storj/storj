// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// ReadFlags will read in and bind flags for viper and pflag
func readFlags() {
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	err := viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		log.Print("error parsing command line flags into viper:", err)
	}
}

// get default config folder
func configPath() string {
	home, _ := homedir.Dir()
	return filepath.Join(home, ".storj")
}

// get default config file
func defaultConfigFile(name string) string {
	return filepath.Join(configPath(), name)
}

func generateConfig() error {
	err := viper.WriteConfigAs(defaultConfigFile("main.json"))
	return err
}

// ConfigEnvironment will read in command line flags, set the name of the config file,
// then look for configs in the current working directory and in $HOME/.storj
func ConfigEnvironment() (*viper.Viper, error) {
	viper.SetEnvPrefix("storj")
	viper.AutomaticEnv()
	viper.SetConfigName("main")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.AddConfigPath(configPath())

	// Override default config with a specific config
	cfgFile := flag.String("config", "", "config file")
	generate := flag.Bool("generate", false, "generate a default config in ~/.storj")

	// if that file exists, set it as the config instead of reading in from default locations
	if *cfgFile != "" && fileExists(*cfgFile) {
		viper.SetConfigFile(*cfgFile)
	}

	err := viper.ReadInConfig()

	if err != nil {
		log.Print("could not read config file; defaulting to command line flags for configuration")
	}

	readFlags()

	if *generate == true {
		err := generateConfig()
		if err != nil {
			log.Print("unable to generate config file.", err)
		}
	}

	v := viper.GetViper()
	return v, nil
}

// check if file exists, handle error correctly if it doesn't
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatalf("failed to check for file existence: %v", err)
	}
	return true
}

// Execute runs a *cobra.Command and sets up Storj-wide process configuration
// like a configuration file and logging.
func Execute(cmd *cobra.Command) {
	cobra.OnInitialize(func() {
		_, err := ConfigEnvironment()
		if err != nil {
			log.Fatal("error configuring environment", err)
		}
	})

	Must(cmd.Execute())
}

// Main runs a Service
func Main(configFn func() (*viper.Viper, error), s ...Service) error {
	if _, err := configFn(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		return err
	}
}

// Must checks for errors
func Must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
