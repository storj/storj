// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "storj.io/storj/protos/netstate"
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

	conn, err := grpc.Dial(port, grpc.WithInsecure())
	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
	}

	client := proto.NewNetStateClient(conn)

	logger.Debug(fmt.Sprintf("client dialed port %s", port))

	ctx := context.Background()

	// Example pointer paths to put
	pr1 := proto.PutRequest{
		Path: []byte("welcome/to/my/pointer/journey"),
		Pointer: &proto.Pointer{
			Type: proto.Pointer_INLINE,
			Encryption: &proto.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("granola"),
		},
	}
	pr2 := proto.PutRequest{
		Path: []byte("so/many/pointers"),
		Pointer: &proto.Pointer{
			Type: proto.Pointer_INLINE,
			Encryption: &proto.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("m&ms"),
		},
	}
	pr3 := proto.PutRequest{
		Path: []byte("another/pointer/for/the/pile"),
		Pointer: &proto.Pointer{
			Type: proto.Pointer_INLINE,
			Encryption: &proto.EncryptionScheme{
				EncryptedEncryptionKey: []byte("key"),
				EncryptedStartingNonce: []byte("nonce"),
			},
			InlineSegment: []byte("popcorn"),
		},
	}

	// Example Puts
	_, err = client.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to put", zap.Error(err))
	}
	_, err = client.Put(ctx, &pr2)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to put", zap.Error(err))
	}
	_, err = client.Put(ctx, &pr3)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to put", zap.Error(err))
	}

	// Example Get
	getReq := proto.GetRequest{
		Path: []byte("so/many/pointers"),
	}
	getRes, err := client.Get(ctx, &getReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to get", zap.Error(err))
	}
	pointer := string(getRes.Pointer)
	logger.Debug("get response: " + pointer)

	// Example List
	listReq := proto.ListRequest{
		// This pagination functionality doesn't work yet.
		// The given arguments are placeholders.
		StartingPathKey: []byte("test/pointer/path"),
		Limit:           5,
	}
	listRes, err := client.List(ctx, &listReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to list file paths")
	}
	var stringList []string
	for _, pathByte := range listRes.Paths {
		stringList = append(stringList, string(pathByte))
	}
	logger.Debug("listed paths: " + strings.Join(stringList, ", "))

	// Example Delete
	delReq := proto.DeleteRequest{
		Path: []byte("welcome/to/my/pointer/journey"),
	}
	_, err = client.Delete(ctx, &delReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to delete: " + string(delReq.Path))
	}
}
