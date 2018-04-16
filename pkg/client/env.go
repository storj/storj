// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"strings"

	"github.com/spf13/viper"
)

// DefaultURL of the Storj Bridge API endpoint
const DefaultURL = "https://api.storj.io"

func init() {
	viper.SetEnvPrefix("storj")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
	viper.SetDefault("bridge", DefaultURL)
}

// Env contains parameters for accessing the Storj network
type Env struct {
	URL      string
	User     string
	Password string
	Mnemonic string
}

// NewEnv creates new Env struct with default values
func NewEnv() Env {
	return Env{
		URL:      viper.GetString("bridge"),
		User:     viper.GetString("bridge-user"),
		Password: sha256Sum(viper.GetString("bridge-pass")),
		Mnemonic: viper.GetString("encryption-key"),
	}
}
