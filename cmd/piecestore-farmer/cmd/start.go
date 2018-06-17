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
	"net"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // sqlite driver
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"storj.io/storj/cmd/piecestore-farmer/utils"
	"storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	pb "storj.io/storj/protos/piecestore"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a farmer node by ID",
	Long: "Start farmer node by ID using farmer node config values",
	RunE: startNode,
}

func init() {
	var err error
	rootCmd.AddCommand(startCmd)

	sugar, err = utils.NewLogger()
	if err != nil {
		log.Fatalf("%v", err)
	}

	home, err = homedir.Dir()
	if err != nil {
		sugar.Fatalf("%v", err)
	}
}

// startNode starts a farmer node by ID
func startNode(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	if len(args) == 0 {
		return errs.New("no id specified")
	}

	_, _ = utils.SetConfigPath(home, args[0])

	err := viper.ReadInConfig()
	if err != nil {
		return err
	}

	config := utils.GetConfigValues()

	dbPath := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "/ttl-data.db")
	dataDir := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "/piece-store-data/")

	err = os.MkdirAll(config.PieceStoreDir, 0700)
	if err != nil {
		return err
	}

	fileInfo, err := os.Stat(config.PieceStoreDir)
	if err != nil {
		return err
	}

	_, err = utils.ConnectToKad(ctx, config.NodeID, config.PsHost, config.KadListenPort, fmt.Sprintf("%s:%s", config.KadHost, config.KadPort))
	if err != nil {
		return err
	}

	if fileInfo.IsDir() != true {
		return errs.New("pieceStoreDir is not a directory")
	}

	ttlDB, err := ttl.NewTTL(dbPath)
	if err != nil {
		return err
	}

	// create a listener on TCP port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", config.PsPort))
	if err != nil {
		return err
	}

	defer lis.Close()

	// create a server instance
	s := server.Server{PieceStoreDir: dataDir, DB: ttlDB}

	// create a gRPC server object
	grpcServer := grpc.NewServer()

	// attach the api service to the server
	pb.RegisterPieceStoreRoutesServer(grpcServer, &s)

	// routinely check DB and delete expired entries
	go func() {
		err := s.DB.DBCleanup(dataDir)
		sugar.Fatalf("Error in DBCleanup: %v", err)
	}()

	// start the server
	if err := grpcServer.Serve(lis); err != nil {
		sugar.Fatalf("failed to serve: %s", err)
	}

	return nil
}
