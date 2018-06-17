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
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // sqlite driver
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/piecestore-farmer/utils"
)

// deleteCmd represents the delete command
var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a farmer node by ID",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: deleteNode,
}

func init() {
	var err error
	rootCmd.AddCommand(deleteCmd)

	sugar, err = utils.NewLogger()
	if err != nil {
		log.Fatalf("%v", err)
	}

	home, err = homedir.Dir()
	if err != nil {
		sugar.Fatalf("%v", err)
	}
}

// deleteNode deletes a farmer node by ID
func deleteNode(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errs.New("no id specified")
	}

	nodeID := args[0]

	configDir, configFile := utils.SetConfigPath(home, nodeID)

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return errs.New("Invalid node id. Config file does not exist")
	}

	viper.SetConfigName(nodeID)
	viper.AddConfigPath(configDir)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}

	// get folder for stored data
	piecestoreDir := viper.GetString("piecestore.dir")
	piecestoreDir = filepath.Join(piecestoreDir, fmt.Sprintf("store-%s", nodeID))

	// remove all folders and files stored on node
	if err := os.RemoveAll(piecestoreDir); err != nil {
		return err
	}

	// delete node config
	err := os.Remove(configFile)
	if err != nil {
		return err
	}

	sugar.Infof("Deleted node: %s", nodeID)

}
