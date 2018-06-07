// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
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
	//pr1 passes with api creds
	for i := 0; i < 100; i++ {

		pr1 := proto.PutRequest{
			Path: []byte((strconv.Itoa(i) + "welcome/to/my/pointer/journey")),
			Pointer: &proto.Pointer{
				Type: proto.Pointer_INLINE,
				Encryption: &proto.EncryptionScheme{
					EncryptedEncryptionKey: []byte("key"),
					EncryptedStartingNonce: []byte("nonce"),
				},
				InlineSegment: []byte("granola"),
			},
			APIKey: []byte("abc123"),
		}
		// pr2 passes with api creds
		pr2 := proto.PutRequest{
			Path: []byte(strconv.Itoa(i) + "so/many/pointers"),
			Pointer: &proto.Pointer{
				Type: proto.Pointer_INLINE,
				Encryption: &proto.EncryptionScheme{
					EncryptedEncryptionKey: []byte("key"),
					EncryptedStartingNonce: []byte("nonce"),
				},
				InlineSegment: []byte("m&ms"),
			},
			APIKey: []byte("abc123"),
		}
		// pr3 fails api creds
		pr3 := proto.PutRequest{
			Path: []byte(strconv.Itoa(i) + "another/pointer/for/the/pile"),
			Pointer: &proto.Pointer{
				Type: proto.Pointer_INLINE,
				Encryption: &proto.EncryptionScheme{
					EncryptedEncryptionKey: []byte("key"),
					EncryptedStartingNonce: []byte("nonce"),
				},
				InlineSegment: []byte("popcorn"),
			},
			APIKey: []byte("abc13"),
		}

		// Example Puts
		// puts passes api creds
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
	}

	// Example Get
	// get passes api creds
	getReq := proto.GetRequest{
		Path:   []byte("so/many/pointers"),
		APIKey: []byte("abc123"),
	}
	getRes, err := client.Get(ctx, &getReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to get", zap.Error(err))
	} else {
		pointer := string(getRes.Pointer)
		logger.Debug("get response: " + pointer)
	}

	// Example List
	// list passes api creds
	listReq := proto.ListRequest{
		// This pagination functionality doesn't work yet.
		// The given arguments are placeholders.
		StartingPathKey: []byte("test/pointer/path"),
		Limit:           5,
		APIKey:          []byte("abc123"),
	}

	listRes, err := client.List(ctx, &listReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to list file paths")
	} else {
		var stringList []string
		for _, pathByte := range listRes.Paths {
			stringList = append(stringList, string(pathByte))
		}
		logger.Debug("listed paths: " + strings.Join(stringList, ", "))
	}

	// Example Delete
	// delete passes api creds
	delReq := proto.DeleteRequest{
		Path:   []byte("welcome/to/my/pointer/journey"),
		APIKey: []byte("abc123"),
	}
	_, err = client.Delete(ctx, &delReq)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to delete: " + string(delReq.Path))
	}
}
