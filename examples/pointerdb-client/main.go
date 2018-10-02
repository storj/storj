// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	p "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/storage/meta"
)

var (
	pointerdbClientPort string
	ctx                 = context.Background()
)

func initializeFlags() {
	flag.StringVar(&pointerdbClientPort, "pointerdbPort", ":8080", "this is your port")
	flag.Parse()
}

func main() {
	initializeFlags()

	logger, _ := zap.NewDevelopment()
	defer printError(logger.Sync)

	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		logger.Error("Failed to create certificate authority: ", zap.Error(err))
		os.Exit(1)
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		logger.Error("Failed to create full identity: ", zap.Error(err))
		os.Exit(1)
	}
	APIKey := "abc123"
	client, err := pdbclient.NewClient(identity, pointerdbClientPort, APIKey)

	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
		os.Exit(1)
	}

	logger.Debug(fmt.Sprintf("client dialed port %s", pointerdbClientPort))
	ctx := context.Background()

	// Example parameters to pass into API calls
	var path = p.New("fold1/fold2/fold3/file.txt")
	pointer := &pb.Pointer{
		Type:          pb.Pointer_INLINE,
		InlineSegment: []byte("popcorn"),
	}

	// Example Put1
	err = client.Put(ctx, path, pointer)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}

	// Example Put2
	err = client.Put(ctx, p.New("fold1/fold2"), pointer)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}

	// Example Get
	getRes, err := client.Get(ctx, path)

	if err != nil {
		logger.Error("couldn't GET pointer from db", zap.Error(err))
	} else {
		logger.Info("Success: got Pointer from db",
			zap.String("pointer", getRes.String()),
		)
	}

	// Example List with pagination
	prefix := p.New("fold1")
	items, more, err := client.List(ctx, prefix, nil, nil, true, 1, meta.None)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to list file paths", zap.Error(err))
	} else {
		var stringList []string
		for _, item := range items {
			stringList = append(stringList, item.Path.String())
		}
		logger.Debug("Success: listed paths: " + strings.Join(stringList, ", ") + "; more: " + fmt.Sprintf("%t", more))
	}

	// Example Delete
	err = client.Delete(ctx, path)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("Error in deleteing file from db", zap.Error(err))
	} else {
		logger.Debug("Success: file is deleted from db")
	}
}

func printError(fn func() error) {
	err := fn()
	if err != nil {
		fmt.Println(err)
	}
}
