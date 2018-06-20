// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"net"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3" // sqlite driver
	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
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
	Long:  "Start farmer node by ID using farmer node config values",
	RunE:  startNode,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

// startNode starts a farmer node by ID
func startNode(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	home, err := homedir.Dir()
	if err != nil {
		return err
	}

	if len(args) == 0 {
		return errs.New("no id specified")
	}

	utils.SetConfigPath(home, args[0])

	err = viper.ReadInConfig()
	if err != nil {
		return err
	}

	config := utils.GetConfigValues()

	dbPath := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "ttl-data.db")
	dataDir := filepath.Join(config.PieceStoreDir, fmt.Sprintf("store-%s", config.NodeID), "piece-store-data")

	_, err = utils.ConnectToKad(ctx, config.NodeID, config.PsHost, config.KadListenPort, fmt.Sprintf("%s:%s", config.KadHost, config.KadPort))
	if err != nil {
		return err
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
		zap.S().Fatalf("Error in DBCleanup: %v", err)
	}()

	// start the server
	if err := grpcServer.Serve(lis); err != nil {
		zap.S().Fatalf("failed to serve: %s", err)
	}

	return nil
}
