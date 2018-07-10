// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"storj.io/storj/pkg/piecestore/rpc/server"
)

// SetConfigPath sets and returns viper config directory and filepath
func SetConfigPath(fileName string) (configDir, configFile string, err error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", "", err
	}

	configDir = filepath.Join(home, ".storj")
	configFile = filepath.Join(configDir, fileName+".yaml")

	viper.SetConfigFile(configFile)

	return configDir, configFile, nil
}

// GetConfigValues returns a struct with config file values
func GetConfigValues() server.Config {
	config := server.Config{
		NodeID:        viper.GetString("piecestore.id"),
		PsHost:        viper.GetString("piecestore.host"),
		PsPort:        viper.GetString("piecestore.port"),
		KadListenPort: viper.GetString("kademlia.listen.port"),
		KadPort:       viper.GetString("kademlia.port"),
		KadHost:       viper.GetString("kademlia.host"),
		PieceStoreDir: viper.GetString("piecestore.dir"),
	}
	return config
}
