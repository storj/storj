// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/spf13/cobra"
)

func setenv(key, value string) func() {
	old := os.Getenv(key)
	_ = os.Setenv(key, value)
	return func() { _ = os.Setenv(key, old) }
}

func TestExec_PropagatesSettings(t *testing.T) {
	// Set up a command that does nothing.
	cmd := &cobra.Command{RunE: func(cmd *cobra.Command, args []string) error { return nil }}

	// Define a config struct and some flags.
	var config struct {
		X int `default:"0"`
	}
	Bind(cmd, &config)
	y := cmd.Flags().Int("y", 0, "y flag (command)")
	z := flag.Int("z", 0, "z flag (stdlib)")

	// Set some environment variables for viper.
	defer setenv("STORJ_X", "1")()
	defer setenv("STORJ_Y", "2")()
	defer setenv("STORJ_Z", "3")()

	// Run the command through the exec call.
	Exec(cmd)

	// Check that the variables are now bound.
	require.Equal(t, 1, config.X)
	require.Equal(t, 2, *y)
	require.Equal(t, 3, *z)
}
