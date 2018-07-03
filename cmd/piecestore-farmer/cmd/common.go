// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"golang.org/x/net/context"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
)

// Config stores values from a farmer node config file
type Config struct {
	NodeID        string
	PsHost        string
	PsPort        string
	KadListenPort string
	KadPort       string
	KadHost       string
	PieceStoreDir string
}

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
func GetConfigValues() Config {
	config := Config{
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

// ConnectToKad joins the Kademlia network
func ConnectToKad(ctx context.Context, id, ip, kadListenPort, kadAddress string) (*kademlia.Kademlia, error) {
	node := proto.Node{
		Id: id,
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   kadAddress,
		},
	}

	kad, err := kademlia.NewKademlia(kademlia.StringToNodeID(id), []proto.Node{node}, ip, kadListenPort)
	if err != nil {
		return nil, errs.New("Failed to instantiate new Kademlia: %s", err.Error())
	}

	if err := kad.ListenAndServe(); err != nil {
		return nil, errs.New("Failed to ListenAndServe on new Kademlia: %s", err.Error())
	}

	if err := kad.Bootstrap(ctx); err != nil {
		return nil, errs.New("Failed to Bootstrap on new Kademlia: %s", err.Error())
	}

	return kad, nil
}
