// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package cmd_test

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/storj"
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

	err := setupCmd.Flags().Set(flagName, "ahost")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "ahost:7777", setupCmd.Flags().Lookup("satellite-addr").Value.String(),
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

	err := setupCmd.Flags().Set(flagName, "ahost:7778")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "ahost:7778", setupCmd.Flags().Lookup(flagName).Value.String(),
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

func TestDefaultPortAppliedToSatelliteAddrWithPortColonButNoPort(t *testing.T) {
	setupCmd := &cobra.Command{
		Use:         "setup",
		Short:       "Create an uplink config file",
		RunE:        nil,
		Annotations: map[string]string{"type": "setup"},
	}
	flagName := "satellite-addr"
	defaultValue := "localhost:7777"
	setupCmd.Flags().String(flagName, defaultValue, "")

	err := setupCmd.Flags().Set(flagName, "ahost:")
	assert.NoError(t, err)

	err = cmd.ApplyDefaultHostAndPortToAddrFlag(setupCmd, flagName)
	assert.NoError(t, err)

	assert.Equal(t, "ahost:7777", setupCmd.Flags().Lookup("satellite-addr").Value.String(),
		"satellite-addr should contain default port when no port specified")
}

func TestSaveEncryptionKey(t *testing.T) {
	var expectedKey *storj.Key
	{
		key := make([]byte, rand.Intn(20)+1)
		_, err := rand.Read(key)
		require.NoError(t, err)
		expectedKey = storj.NewKey(key)
	}

	t.Run("ok", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("storj-test-cmd-uplink", "encryption.key")
		err := cmd.SaveEncryptionKey(expectedKey, filename)
		require.NoError(t, err)

		key, err := ioutil.ReadFile(filename)
		require.NoError(t, err)
		assert.Equal(t, expectedKey, storj.NewKey(key))
	})

	t.Run("error: unexisting dir", func(t *testing.T) {
		// Create a directory and remove it for making sure that the path doesn't
		// exist
		ctx := testcontext.New(t)
		dir := ctx.Dir("storj-test-cmd-uplink")
		ctx.Cleanup()

		filename := filepath.Join(dir, "enc.key")
		err := cmd.SaveEncryptionKey(expectedKey, filename)
		require.Errorf(t, err, "directory path doesn't exist")
	})

	t.Run("error: file already exists", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()
		filename := ctx.File("encryption.key")
		require.NoError(t, ioutil.WriteFile(filename, nil, os.FileMode(0600)))

		err := cmd.SaveEncryptionKey(expectedKey, filename)
		require.Errorf(t, err, "file key already exists")
	})
}
