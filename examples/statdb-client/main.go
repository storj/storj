// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"storj.io/storj/pkg/provider"
	proto "storj.io/storj/pkg/statdb/proto"
	"storj.io/storj/pkg/statdb/sdbclient"
)

var (
	port   string
	apiKey = []byte("")
	ctx    = context.Background()
)

func initializeFlags() {
	flag.StringVar(&port, "port", ":7777", "port")
	flag.Parse()
}

func printNodeStats(ns proto.NodeStats, logger zap.Logger) {
	nodeID := ns.NodeId
	latency90 := ns.Latency_90
	auditSuccess := ns.AuditSuccessRatio
	uptime := ns.UptimeRatio
	logStr := fmt.Sprintf("NodeID: %s, Latency (90th percentile): %d, Audit Success Ratio: %g, Uptime Ratio: %g", nodeID, latency90, auditSuccess, uptime)
	logger.Info(logStr)
}

func main() {
	initializeFlags()

	logger, _ := zap.NewDevelopment()
	defer printError(logger.Sync)

	idents := make([]*provider.FullIdentity, 3)
	for i := range idents {
		var err error
		idents[i], err = provider.NewFullIdentity(ctx, 12, 4)
		if err != nil {
			logger.Error("Failed to create certificate authority: ", zap.Error(err))
			os.Exit(1)
		}
	}
	client, err := sdbclient.NewClient(idents[0], port, apiKey)
	if err != nil {
		logger.Error("Failed to create sdbclient: ", zap.Error(err))
	}

	logger.Debug(fmt.Sprintf("client dialed port %s", port))

	// Test farmers
	farmer1 := proto.Node{
		Id:             idents[1].ID,
		UpdateAuditSuccess: false,
		UpdateUptime:       false,
	}
	farmer2 := proto.Node{
		Id:             idents[2].ID,
		UpdateAuditSuccess: false,
		UpdateUptime:       false,
	}

	// Example Creates
	err = client.Create(ctx, farmer1.Id)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to create", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("Farmer 1 created successfully")

	err = client.Create(ctx, farmer2.Id)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to create", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("Farmer 2 created successfully")

	// Example Updates
	farmer1.AuditSuccess = true
	farmer1.IsUp = true
	farmer1.UpdateAuditSuccess = true
	farmer1.UpdateUptime = true

	nodeStats, err := client.Update(ctx, farmer1.Id, farmer1.AuditSuccess, farmer1.IsUp, nil)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("Farmer 1 after Update")
	printNodeStats(*nodeStats, *logger)

	// Example UpdateBatch
	farmer1.AuditSuccess = false
	farmer1.IsUp = false

	farmer2.AuditSuccess = true
	farmer2.IsUp = true
	farmer2.UpdateAuditSuccess = true
	farmer2.UpdateUptime = true

	nodeList := []*proto.Node{&farmer1, &farmer2}

	statsList, _, err := client.UpdateBatch(ctx, nodeList)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update batch", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("Farmer stats after UpdateBatch")
	for _, statsEl := range statsList {
		printNodeStats(*statsEl, *logger)
	}

	// Example Get
	nodeStats, err = client.Get(ctx, farmer1.Id)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("Farmer 1 after Get 1")
	printNodeStats(*nodeStats, *logger)

	nodeStats, err = client.Get(ctx, farmer2.Id)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update", zap.Error(err))
		os.Exit(1)
	}
	logger.Info("Farmer 2 after Get 2")
	printNodeStats(*nodeStats, *logger)
}

func printError(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}
