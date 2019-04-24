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

func TestSaveEncriptionKey(t *testing.T) {
	expectedKey := make([]byte, rand.Intn(20)+1)
	_, err := rand.Read(expectedKey)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		filename := ctx.File("storj-test-cmd-uplink", ".enc.key")
		err = cmd.SaveEncryptionKey(expectedKey, filename)
		require.NoError(t, err)

		// Read the key from the file to compare that it matches with the saved one.
		file, err := os.Open(filename)
		require.NoError(t, err)
		defer func() { require.NoError(t, file.Close()) }()

		key := make([]byte, len(expectedKey))
		_, err = file.Read(key)
		require.NoError(t, err)
		assert.Equal(t, expectedKey, key)

		// Check that the key file has a read only file permissions for the user
		fileInfo, err := file.Stat()
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0400), fileInfo.Mode().Perm())
	})

	t.Run("error: unexisting dir", func(t *testing.T) {
		// Create a directory and remove it for making sure that the path doesn't
		// exist
		dir, err := ioutil.TempDir("", "storj-test-cmd-uplink")
		require.NoError(t, err)
		err = os.RemoveAll(dir)
		require.NoError(t, err)

		filename := filepath.Join(dir, "enc.key")
		err = cmd.SaveEncryptionKey(expectedKey, filename)
		require.Errorf(t, err, "directory path doesn't exist")
	})

	t.Run("error: file already exists", func(t *testing.T) {
		// Create an empty file
		file, err := ioutil.TempFile("", "storj-test-cmd-uplink-key-*")
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)
		defer func() { require.NoError(t, os.Remove(file.Name())) }()

		err = cmd.SaveEncryptionKey(expectedKey, file.Name())
		require.Errorf(t, err, "file key already exists")
	})
}
