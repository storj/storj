// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/piecestore"
)

// createCmd represents the create command
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new farmer node",
	Long:  "Create a config file and set values for a new farmer node",
	RunE:  createNode,
}

func init() {
	RootCmd.AddCommand(CreateCmd)

	nodeID := pstore.GenerateID()[:20]

	home, err := homedir.Dir()
	if err != nil {
		zap.S().Fatal(err)
	}

	CreateCmd.Flags().String("kademliaHost", "bootstrap.storj.io", "Kademlia server `host`")
	CreateCmd.Flags().String("kademliaPort", "8080", "Kademlia server `port`")
	CreateCmd.Flags().String("kademliaListenPort", "7776", "Kademlia server `listen port`")
	CreateCmd.Flags().String("pieceStoreHost", "127.0.0.1", "Farmer's public ip/host")
	CreateCmd.Flags().String("pieceStorePort", "7777", "`port` where piece store data is accessed")
	CreateCmd.Flags().String("dir", home, "`dir` of drive being shared")

	viper.BindPFlag("kademlia.host", CreateCmd.Flags().Lookup("kademliaHost"))
	viper.BindPFlag("kademlia.port", CreateCmd.Flags().Lookup("kademliaPort"))
	viper.BindPFlag("kademlia.listen.port", CreateCmd.Flags().Lookup("kademliaListenPort"))
	viper.BindPFlag("piecestore.host", CreateCmd.Flags().Lookup("pieceStoreHost"))
	viper.BindPFlag("piecestore.port", CreateCmd.Flags().Lookup("pieceStorePort"))
	viper.BindPFlag("piecestore.dir", CreateCmd.Flags().Lookup("dir"))

	viper.SetDefault("piecestore.id", nodeID)

}

// createNode creates a config file for a new farmer node
func createNode(cmd *cobra.Command, args []string) error {
	home, err := homedir.Dir()
	if err != nil {
		return err
	}

	configDir, configFile := SetConfigPath(home, viper.GetString("piecestore.id"))

	pieceStoreDir := viper.GetString("piecestore.dir")

	err = os.MkdirAll(pieceStoreDir, 0700)
	if err != nil {
		return err
	}

	err = os.MkdirAll(configDir, 0700)
	if err != nil {
		return err
	}

	if _, err := os.Stat(configFile); err == nil {
		return errs.New("Config already exists")
	}

	err = viper.WriteConfigAs(configFile)
	if err != nil {
		return err
	}

	path := viper.ConfigFileUsed()

	zap.S().Info("Config: ", path)
	zap.S().Info("ID: ", viper.GetString("piecestore.id"))

	fmt.Printf("Node %s created\n", viper.GetString("piecestore.id"))

	return nil
}
