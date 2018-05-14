// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	proto "storj.io/storj/protos/netstate"
)

var (
	port string
)

const (
	success string = "success"
)

func initializeFlags() {
	flag.StringVar(&port, "port", ":8080", "port")
	flag.Parse()
}

func main() {
	initializeFlags()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	conn, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
	}

	client := proto.NewNetStateClient(conn)

	ctx := context.Background()

	// Examples file paths to be saved
	fp := proto.FilePath{
		Path:       []byte("welcome/to/my/file/journey"),
		SmallValue: []byte("granola"),
	}
	fp2 := proto.FilePath{
		Path:       []byte("so/many/file/paths"),
		SmallValue: []byte("m&ms"),
	}
	fp3 := proto.FilePath{
		Path:       []byte("another/file/path/for/the/pile"),
		SmallValue: []byte("popcorn"),
	}

	// Example Puts
	putRes, err := client.Put(ctx, &fp)
	if err != nil || putRes.Confirmation != success {
		logger.Error("failed to put", zap.Error(err))
	}
	putRes2, err := client.Put(ctx, &fp2)
	if err != nil || putRes2.Confirmation != success {
		logger.Error("failed to put", zap.Error(err))
	}
	putRes3, err := client.Put(ctx, &fp3)
	if err != nil || putRes3.Confirmation != success {
		logger.Error("failed to put", zap.Error(err))
	}

	// Example Get
	getReq := proto.GetRequest{
		Path: []byte("so/many/file/paths"),
	}
	getRes, err := client.Get(ctx, &getReq)
	if err != nil {
		logger.Error("failed to get", zap.Error(err))
	}
	value := string(getRes.SmallValue)
	logger.Debug("get response: " + value)

	// Example List
	listReq := proto.ListRequest{
		// This Bucket value isn't actually used by List() now,
		// but in the future could be used to select specific
		// buckets to list from.
		Bucket: []byte("files"),
	}
	listRes, err := client.List(ctx, &listReq)
	if err != nil {
		logger.Error("failed to list file paths")
	}
	var stringList []string
	for _, pathByte := range listRes.Filepaths {
		stringList = append(stringList, string(pathByte))
	}
	logger.Debug("listed paths: " + strings.Join(stringList, ", "))

	// Example Delete
	delReq := proto.DeleteRequest{
		Path: []byte("welcome/to/my/file/journey"),
	}
	delRes, err := client.Delete(ctx, &delReq)
	if err != nil {
		logger.Error("failed to delete: 'welcome/to/my/file/journey'")
	}
	if delRes.Confirmation == "success" {
		logger.Debug("deleted: welcome/to/my/file/journey")
	}
}
