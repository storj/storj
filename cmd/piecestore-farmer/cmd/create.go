// Copyright © 2018 Storj Labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"log"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/cmd/piecestore-farmer/utils"
)

var nodeID string
var home string
var sugar *zap.SugaredLogger

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new farmer node",
	Long:  "Create a config file and set values for a new farmer node",
	RunE:  createNode,
}

func init() {
	var err error
	rootCmd.AddCommand(createCmd)

	sugar, err = utils.NewLogger()
	if err != nil {
		log.Fatalf("%v", err)
	}

	nodeID, err = utils.NewID()
	if err != nil {
		sugar.Fatalf("%v", err)
	}

	home, err = homedir.Dir()
	if err != nil {
		sugar.Fatalf("%v", err)
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

	viper.SetDefault("piecestore.id", nodeID)
}

// createNode creates a config file for a new farmer node
func createNode(cmd *cobra.Command, args []string) error {
	configDir, configFile := utils.SetConfigPath(home, nodeID)

	err := os.MkdirAll(configDir, 0700)
	if err != nil {
		return err
	}

	_, err = os.Stat(configFile)
	if os.IsExist(err) {
		return errs.New("Config already exists")
	}

	// Create empty file at configPath
	_, err = os.Create(configFile)
	if err != nil {
		return err
	}

	err = viper.WriteConfig()
	if err != nil {
		return err
	}

	path := viper.ConfigFileUsed()

	sugar.Infof("Config: %s\n", path)
	sugar.Infof("ID: %s\n", nodeID)

	return nil
}
