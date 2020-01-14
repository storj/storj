// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"io/ioutil"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
)

func setenv(key, value string) func() {
	old := os.Getenv(key)
	_ = os.Setenv(key, value)
	return func() { _ = os.Setenv(key, old) }
}

var testZ = flag.Int("z", 0, "z flag (stdlib)")

func TestExec_PropagatesSettings(t *testing.T) {
	// Set up a command that does nothing.
	cmd := &cobra.Command{RunE: func(cmd *cobra.Command, args []string) error { return nil }}

	// Define a config struct and some flags.
	var config struct {
		X int `default:"0"`
	}
	Bind(cmd, &config)
	y := cmd.Flags().Int("y", 0, "y flag (command)")

	// Set some environment variables for viper.
	defer setenv("STORJ_X", "1")()
	defer setenv("STORJ_Y", "2")()
	defer setenv("STORJ_Z", "3")()

	// Run the command through the exec call.
	Exec(cmd)

	// Check that the variables are now bound.
	require.Equal(t, 1, config.X)
	require.Equal(t, 2, *y)
	require.Equal(t, 3, *testZ)
}

func TestHidden(t *testing.T) {
	// Set up a command that does nothing.
	cmd := &cobra.Command{RunE: func(cmd *cobra.Command, args []string) error { return nil }}

	// Define a config struct with a hidden field.
	var config struct {
		W int `default:"0" hidden:"false"`
		X int `default:"0" hidden:"true"`
		Y int `releaseDefault:"1" devDefault:"0" hidden:"true"`
		Z int `default:"1"`
	}
	Bind(cmd, &config)

	// Setup test config file
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testConfigFile := ctx.File("testconfig.yaml")

	// Run the command through the exec call.
	Exec(cmd)

	// Ensure that the file saves only the necessary data.
	err := SaveConfig(cmd, testConfigFile)
	require.NoError(t, err)

	actualConfigFile, err := ioutil.ReadFile(testConfigFile)
	require.NoError(t, err)

	expectedConfigW := "# w: 0"
	expectedConfigZ := "# z: 1"
	require.Contains(t, string(actualConfigFile), expectedConfigW)
	require.Contains(t, string(actualConfigFile), expectedConfigZ)
	require.NotContains(t, string(actualConfigFile), "# y: ")
	require.NotContains(t, string(actualConfigFile), "# x: ")
}
