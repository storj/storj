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
	"storj.io/storj/pkg/kademlia"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new farmer node",
	Long:  "Create a config file and set values for a new farmer node",
	RunE:  createNode,
}

func init() {
	RootCmd.AddCommand(createCmd)

	nodeID, err := kademlia.NewID()
	if err != nil {
		zap.S().Fatal(err)
	}

	home, err := homedir.Dir()
	if err != nil {
		zap.S().Fatal(err)
	}

	createCmd.Flags().String("kademliaHost", "bootstrap.storj.io", "Kademlia server `host`")
	createCmd.Flags().String("kademliaPort", "8080", "Kademlia server `port`")
	createCmd.Flags().String("kademliaListenPort", "7776", "Kademlia server `listen port`")
	createCmd.Flags().String("pieceStoreHost", "127.0.0.1", "Farmer's public ip/host")
	createCmd.Flags().String("pieceStorePort", "7777", "`port` where piece store data is accessed")
	createCmd.Flags().String("dir", home, "`dir` of drive being shared")

	viper.BindPFlag("kademlia.host", createCmd.Flags().Lookup("kademliaHost"))
	viper.BindPFlag("kademlia.port", createCmd.Flags().Lookup("kademliaPort"))
	viper.BindPFlag("kademlia.listen.port", createCmd.Flags().Lookup("kademliaListenPort"))
	viper.BindPFlag("piecestore.host", createCmd.Flags().Lookup("pieceStoreHost"))
	viper.BindPFlag("piecestore.port", createCmd.Flags().Lookup("pieceStorePort"))
	viper.BindPFlag("piecestore.dir", createCmd.Flags().Lookup("dir"))

	viper.SetDefault("piecestore.id", nodeID.String())
}

// createNode creates a config file for a new farmer node
func createNode(cmd *cobra.Command, args []string) error {
	configDir, configFile, err := SetConfigPath(viper.GetString("piecestore.id"))
	if err != nil {
		return err
	}

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

	fmt.Printf("Node %s created\n", viper.GetString("piecestore.id"))
	fmt.Println("Config: ", path)

	return nil
}
