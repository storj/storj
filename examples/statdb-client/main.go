// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "storj.io/storj/protos/statdb"
)

var (
	port   string
	apiKey = []byte("abc123")
)

func initializeFlags() {
	flag.StringVar(&port, "port", ":8080", "port")
	flag.Parse()
}

func printNodeStats(ns proto.NodeStats, logger zap.Logger) {
	nodeId := ns.NodeId
	latency90 := ns.Latency_90
	auditSuccess := ns.AuditSuccessRatio
	uptime := ns.UptimeRatio
	logStr := fmt.Sprintf("NodeID: %s, Latency (90th percentile): %d, Audit Success Ratio: %d, Uptime Ratio: %d", nodeId, latency90, auditSuccess, uptime)
	logger.Info(logStr)
}

func main() {
	initializeFlags()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	conn, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
	}

	client := proto.NewStatDBClient(conn)

	logger.Debug(fmt.Sprintf("client dialed port %s", port))

	ctx := context.Background()

	// Test farmers
	farmer1 := proto.Node{
		NodeId:             "nodeid1",
		UpdateLatency:      false,
		UpdateAuditSuccess: false,
		UpdateUptime:       false,
	}
	farmer2 := proto.Node{
		NodeId:             "nodeid2",
		UpdateLatency:      false,
		UpdateAuditSuccess: false,
		UpdateUptime:       false,
	}

	// Example Creates
	createReq1 := proto.CreateRequest{
		Node:   &farmer1,
		ApiKey: apiKey,
	}

	createReq2 := proto.CreateRequest{
		Node:   &farmer2,
		ApiKey: apiKey,
	}

	createRes1, err := client.Create(ctx, &createReq1)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to create", zap.Error(err))
	}
	logger.Info("Farmer 1 after Create 1")
	printNodeStats(*createRes1.Stats, *logger)

	createRes2, err := client.Create(ctx, &createReq2)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to create", zap.Error(err))
	}
	logger.Info("Farmer 2 after Create 2")
	printNodeStats(*createRes2.Stats, *logger)

	// Example Updates
	latencyVals := []int64{205, 1671, 370, 895, 1848, 1436, 1941, 564, 1829, 1203, 913, 1795, 1017, 1282, 1231, 1497, 1322, 382, 1417, 1013}
	farmer1.LatencyList = latencyVals
	farmer1.AuditSuccess = true
	farmer1.IsUp = true
	farmer1.UpdateLatency = true
	farmer1.UpdateAuditSuccess = true
	farmer1.UpdateUptime = true

	updateReq := proto.UpdateRequest{
		Node:   &farmer1,
		ApiKey: apiKey,
	}

	updateRes, err := client.Update(ctx, &updateReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update", zap.Error(err))
	}
	logger.Info("Farmer 1 after Update")
	printNodeStats(*updateRes.Stats, *logger)

	// Example UpdateBatch
	farmer1.AuditSuccess = false
	farmer1.IsUp = false
	farmer1.UpdateLatency = false

	farmer2.LatencyList = latencyVals
	farmer2.AuditSuccess = true
	farmer2.IsUp = true
	farmer2.UpdateLatency = true
	farmer2.UpdateAuditSuccess = true
	farmer2.UpdateUptime = true

	nodeList := []*proto.Node{&farmer1, &farmer2}
	updateBatchReq := proto.UpdateBatchRequest{
		NodeList: nodeList,
		ApiKey:   apiKey,
	}

	updateBatchRes, err := client.UpdateBatch(ctx, &updateBatchReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update batch", zap.Error(err))
	}
	logger.Info("Farmer stats after UpdateBatch")
	statsList := updateBatchRes.StatsList
	for _, statsEl := range statsList {
		printNodeStats(*statsEl, *logger)
	}

	// Example Get
	getReq1 := proto.GetRequest{
		NodeId: farmer1.NodeId,
		ApiKey: apiKey,
	}

	getReq2 := proto.GetRequest{
		NodeId: farmer2.NodeId,
		ApiKey: apiKey,
	}

	getRes1, err := client.Get(ctx, &getReq1)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update", zap.Error(err))
	}
	logger.Info("Farmer 1 after Get 1")
	printNodeStats(*getRes1.Stats, *logger)

	getRes2, err := client.Get(ctx, &getReq2)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to update", zap.Error(err))
	}
	logger.Info("Farmer 2 after Get 2")
	printNodeStats(*getRes2.Stats, *logger)
}
