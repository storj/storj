// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	client "storj.io/storj/pkg/netstate"
	proto "storj.io/storj/protos/netstate"
	p "storj.io/storj/pkg/paths"
)

var (
	port string
)

func initializeFlags() {
	flag.StringVar(&port, "port", ":8080", "port")
	flag.Parse()
}

func main() {
	initializeFlags()

	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	nsclient, err := client.NewNetstateClient(port)

	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
	}

	logger.Debug(fmt.Sprintf("client dialed port %s", port))
	ctx := context.Background()

	// Example parameters to pass into API calls
	var path = p.New("fold1/fold2/fold3/file.txt")
	pointer := &proto.Pointer {
		Type: proto.Pointer_INLINE,
		InlineSegment: []byte("popcorn"),
	}
	APIKey := []byte("abc123")

	// Example Put1
	err = nsclient.Put(ctx, path, pointer, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}

	// Example Put2
	err = nsclient.Put(ctx, p.New("fold1/fold2"), pointer, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}

	// Example Get
	getRes, err := nsclient.Get(ctx, path, APIKey)

	if err != nil {
		logger.Error("couldn't GET pointer from db", zap.Error(err))
	} else {
		logger.Info("Success: got Pointer from db",
			zap.String("pointer", getRes.String()),
		)
	}

	// Example List with pagination 
	startingPathKey := p.New("fold1/")
	var limit int64 = 1

	paths, trunc, err := nsclient.List(ctx, startingPathKey, limit, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to list file paths")
	} else {
		var stringList []string
		for _, pathByte := range paths {
			stringList = append(stringList, string(pathByte))
		}
		logger.Debug("Success: listed paths: " + strings.Join(stringList, ", ") + "; truncated: " + fmt.Sprintf("%t", trunc))
	}

	// Example Delete
	err = nsclient.Delete(ctx, path, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("Error in deleteing file from db")
	} else {
		logger.Debug("Success: file is deleted from db")
	}
}
