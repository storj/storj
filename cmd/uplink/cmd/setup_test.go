// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package cmd_test

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/cmd/uplink/cmd"
)

func TestDefaultHostAndPortAppliedToSatelliteAddrWithNoHostOrPort(t *testing.T) {
	setupCmd := &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        nil,
		Annotations: map[string]string{"type": "setup"},
	}
	flagName := "satellite-addr"
	defaultValue := "localhost:7777"
	setupCmd.Flags().String(flagName, defaultValue, "")

	err := setupCmd.Flags().Set(flagName, "")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "localhost:7777", setupCmd.Flags().Lookup("satellite-addr").Value.String(),
		"satellite-addr should contain default port when no port specified")
}

func TestDefaultPortAppliedToSatelliteAddrWithNoPort(t *testing.T) {
	setupCmd := &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        nil,
		Annotations: map[string]string{"type": "setup"},
	}
	flagName := "satellite-addr"
	defaultValue := "localhost:7777"
	setupCmd.Flags().String(flagName, defaultValue, "")

	err := setupCmd.Flags().Set(flagName, "localhost")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "localhost:7777", setupCmd.Flags().Lookup("satellite-addr").Value.String(),
		"satellite-addr should contain default port when no port specified")
}

func TestNoDefaultPortAppliedToSatelliteAddrWithPort(t *testing.T) {
	setupCmd := &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        nil,
		Annotations: map[string]string{"type": "setup"},
	}
	flagName := "satellite-addr"
	defaultValue := "localhost:7777"
	setupCmd.Flags().String(flagName, defaultValue, "")

	err := setupCmd.Flags().Set(flagName, "localhost:7778")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "localhost:7778", setupCmd.Flags().Lookup(flagName).Value.String(),
		"satellite-addr should contain default port when no port specified")
}

func TestDefaultHostAppliedToSatelliteAddrWithNoHost(t *testing.T) {
	setupCmd := &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        nil,
		Annotations: map[string]string{"type": "setup"},
	}
	flagName := "satellite-addr"
	defaultValue := "localhost:7777"
	setupCmd.Flags().String(flagName, defaultValue, "")

	err := setupCmd.Flags().Set(flagName, ":7778")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "localhost:7778", setupCmd.Flags().Lookup("satellite-addr").Value.String(),
		"satellite-addr should contain default port when no port specified")
}
