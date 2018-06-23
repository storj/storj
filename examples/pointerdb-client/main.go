// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"go.uber.org/zap"
	//"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "storj.io/storj/protos/pointerdb"
	nsclient "storj.io/storj/pkg/netstate"
)

var (
	port   string
	apiKey = []byte("abc123")
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

	client := proto.NewPointerDBClient(conn)

	logger.Debug(fmt.Sprintf("client dialed port %s", port))
	ctx := context.Background()

	// Example pointer paths to put
	// the client library creates a put req. object of these items
	// and sends to server
	path:= []byte("another/pointer/for/the/pile, another/two")
	pointer:= &proto.Pointer{
		Type: proto.Pointer_INLINE,
		Encryption: &proto.EncryptionScheme{
			EncryptedEncryptionKey: []byte("key"),
			EncryptedStartingNonce: []byte("nonce"),
		},
		InlineSegment: []byte("popcorn"),
	}
	APIKey:= []byte("abc123")


	// Example Put
	err = nsclient.Put(ctx, path, pointer, APIKey)
	
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("couldn't put pointer in db", zap.Error(err))
	} else {
		logger.Debug("Success: put pointer in db")
	}
	
	// Example Get
	getRes, err := nsclient.Get(ctx, path, APIKey)
	p := "success"

	if err != nil {
		logger.Error("couldn't GET pointer from db", zap.Error(err))
	} else {	
		// WIP; i need to convert a custom type to string, 
		// will work on this later
		fmt.Println(getRes)	
		logger.Info("Success: got Pointer from db",
			zap.String("pointer", p),
		)
	}

	// Example List

	// This pagination functionality doesn't work yet.
	// The given arguments are placeholders.
	startingPathKey:= []byte("test/pointer/path")
	var limit int64 = 5
	
	paths, trunc, err := nsclient.List(ctx, startingPathKey, limit, APIKey)

	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to list file paths")
	} else {
		var stringList []string
		for _, pathByte := range paths {
			stringList = append(stringList, string(pathByte))
		}
		logger.Debug("Success: listed paths: " + strings.Join(stringList, ", "))
		fmt.Println(trunc)
	}


	// Example Delete
	_, err = nsclient.Delete(ctx, path, APIKey)
	
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("Error in deleteing file from db")
	} else {
		logger.Debug("Success: file is deleted from db")
	}
}
