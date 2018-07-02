// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestCreate tests the farmer CLI create command
func TestCreate(t *testing.T) {
	home, err := homedir.Dir()
	if err != nil {
		t.Error(err.Error())
	}

	tempFile, err := ioutil.TempFile(os.TempDir(), "")
	if err != nil {
		t.Error(err.Error())
	}

	defer os.Remove(tempFile.Name())

	tests := []struct {
		it             string
		expectedConfig Config
		args           []string
		err            string
	}{
		{
			it: "should successfully write config with default values",
			expectedConfig: Config{
				NodeID:        "",
				PsHost:        "127.0.0.1",
				PsPort:        "7777",
				KadListenPort: "7776",
				KadPort:       "8080",
				KadHost:       "bootstrap.storj.io",
				PieceStoreDir: home,
			},
			args: []string{
				"create",
			},
			err: "",
		},
		{
			it: "should successfully write config with flag values",
			expectedConfig: Config{
				NodeID:        "",
				PsHost:        "123.4.5.6",
				PsPort:        "1234",
				KadListenPort: "4321",
				KadPort:       "4444",
				KadHost:       "hack@theplanet.com",
				PieceStoreDir: os.TempDir(),
			},
			args: []string{
				"create",
				fmt.Sprintf("--pieceStoreHost=%s", "123.4.5.6"),
				fmt.Sprintf("--pieceStorePort=%s", "1234"),
				fmt.Sprintf("--kademliaListenPort=%s", "4321"),
				fmt.Sprintf("--kademliaPort=%s", "4444"),
				fmt.Sprintf("--kademliaHost=%s", "hack@theplanet.com"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			err: "",
		},
		{
			it: "should err when a config with identical ID already exists",
			expectedConfig: Config{
				NodeID:        "",
				PsHost:        "123.4.5.6",
				PsPort:        "1234",
				KadListenPort: "4321",
				KadPort:       "4444",
				KadHost:       "hack@theplanet.com",
				PieceStoreDir: home,
			},
			args: []string{
				"create",
				fmt.Sprintf("--dir=%s", home),
			},
			err: "Config already exists",
		},
		{
			it: "should err when pieceStoreDir is not a directory",
			expectedConfig: Config{
				NodeID:        "",
				PsHost:        "123.4.5.6",
				PsPort:        "1234",
				KadListenPort: "4321",
				KadPort:       "4444",
				KadHost:       "hack@theplanet.com",
				PieceStoreDir: tempFile.Name(),
			},
			args: []string{
				"create",
				fmt.Sprintf("--dir=%s", tempFile.Name()),
			},
			err: fmt.Sprintf("mkdir %s: not a directory", tempFile.Name()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			RootCmd.SetArgs(tt.args)

			path := filepath.Join(home, ".storj", viper.GetString("piecestore.id")+".yaml")

			if tt.err == "Config already exists" {
				_, err := os.Create(path)
				if err != nil {
					t.Error(err.Error())
					return
				}
			}

			defer os.Remove(path)

			err := RootCmd.Execute()
			if tt.err != "" {
				if err != nil {
					assert.Equal(tt.err, err.Error())
					return
				}
			} else if err != nil {
				t.Error(err.Error())
				return
			}

			viper.SetConfigFile(path)

			err = viper.ReadInConfig()
			if err != nil {
				t.Error(err.Error())
				return
			}

			tt.expectedConfig.NodeID = viper.GetString("piecestore.id")

			assert.Equal(tt.expectedConfig, GetConfigValues())

			return
		})
	}
}

// TestStart tests the farmer CLI start command
func TestStart(t *testing.T) {
	home, err := homedir.Dir()
	if err != nil {
		t.Error(err.Error())
	}

	tests := []struct {
		it         string
		createArgs []string
		startArgs  []string
		err        string
	}{
		{
			it: "should err with no ID specified",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", "7776"),
				fmt.Sprintf("--pieceStoreHost=%s", "127.0.0.1"),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			startArgs: []string{
				"start",
			},
			err: "No ID specified",
		},
		{
			it: "should err with invalid Farmer IP",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", "7776"),
				fmt.Sprintf("--pieceStoreHost=%s", "123"),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			startArgs: []string{
				"start",
				viper.GetString("piecestore.id"),
			},
			err: "Failed to instantiate new Kademlia: lookup 123: no such host",
		},
		{
			it: "should err with missing Kademlia Listen Port",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", ""),
				fmt.Sprintf("--pieceStoreHost=%s", "127.0.0.1"),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			startArgs: []string{
				"start",
				viper.GetString("piecestore.id"),
			},
			err: "Failed to instantiate new Kademlia: node error: must specify port in request to NewKademlia",
		},
		{
			it: "should err with missing IP",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", "7776"),
				fmt.Sprintf("--pieceStoreHost=%s", ""),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			startArgs: []string{
				"start",
				viper.GetString("piecestore.id"),
			},
			err: "Failed to instantiate new Kademlia: lookup : no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			RootCmd.SetArgs(tt.createArgs)

			err = RootCmd.Execute()
			if err != nil {
				t.Error(err.Error())
				return
			}

			path := filepath.Join(home, ".storj", viper.GetString("piecestore.id")+".yaml")
			defer os.Remove(path)

			RootCmd.SetArgs(tt.startArgs)

			err := RootCmd.Execute()
			if tt.err != "" {
				if err != nil {
					assert.Equal(tt.err, err.Error())
				}
			} else if err != nil {
				t.Error(err.Error())
				return
			}
		})
	}
}

// TestDelete tests the farmer CLI delete command
func TestDelete(t *testing.T) {
	home, err := homedir.Dir()
	if err != nil {
		t.Error(err.Error())
	}

	tests := []struct {
		it         string
		createArgs []string
		deleteArgs []string
		err        string
	}{
		{
			it: "should successfully delete node config",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", "7776"),
				fmt.Sprintf("--pieceStoreHost=%s", "127.0.0.1"),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			deleteArgs: []string{
				"delete",
				viper.GetString("piecestore.id"),
			},
			err: "",
		},
		{
			it: "should err with no ID specified",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", "7776"),
				fmt.Sprintf("--pieceStoreHost=%s", "127.0.0.1"),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			deleteArgs: []string{
				"delete",
			},
			err: "No ID specified",
		},
		{
			it: "should err with no ID specified",
			createArgs: []string{
				"create",
				fmt.Sprintf("--kademliaHost=%s", "bootstrap.storj.io"),
				fmt.Sprintf("--kademliaPort=%s", "8080"),
				fmt.Sprintf("--kademliaListenPort=%s", "7776"),
				fmt.Sprintf("--pieceStoreHost=%s", "127.0.0.1"),
				fmt.Sprintf("--pieceStorePort=%s", "7777"),
				fmt.Sprintf("--dir=%s", os.TempDir()),
			},
			deleteArgs: []string{
				"delete",
				"123",
			},
			err: "Invalid node ID. Config file does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			RootCmd.SetArgs(tt.createArgs)

			err := RootCmd.Execute()
			if err != nil {
				t.Error(err.Error())
				return
			}

			configPath := filepath.Join(home, ".storj", viper.GetString("piecestore.id")+".yaml")
			if _, err := os.Stat(configPath); err != nil {
				t.Error(err.Error())
				return
			}

			if tt.err != "" {
				defer os.Remove(configPath)
			}

			RootCmd.SetArgs(tt.deleteArgs)

			err = RootCmd.Execute()
			if tt.err != "" {
				if err != nil {
					assert.Equal(tt.err, err.Error())
					return
				}
			} else if err != nil {
				t.Error(err.Error())
				return
			}

			// if err is nil, file was not deleted
			if _, err := os.Stat(configPath); err == nil {
				t.Error(err.Error())
				return
			}

			return
		})
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
