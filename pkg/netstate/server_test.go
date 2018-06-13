// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

const (
	API_KEY = "abc123"
)

func TestMain(m *testing.M) {
	viper.SetEnvPrefix("API")
	os.Setenv("API_KEY", API_KEY)
	viper.AutomaticEnv()
	os.Exit(m.Run())
}
