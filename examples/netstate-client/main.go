// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"fmt"
	//"strings"

	"go.uber.org/zap"
	//"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "storj.io/storj/protos/netstate"
	client "storj.io/storj/pkg/netstate/client"
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

	nsclient, err := client.NewNetStateClient(port)
	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
	}

	logger.Debug(fmt.Sprintf("client dialed port %s", port))
	ctx := context.Background()

	// Example pointer paths to put
	// the client library creates a put req. object of these items
	// and sends to server
	path:= []byte("another/pointer/for/the/pile")
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
		logger.Error("couldn't PUT pointer in db", zap.Error(err))
	} else {
		logger.Debug("Successfully PUT pointer in db")
	}
	
	// Example Get
	res, err := nsclient.Get(ctx, path, APIKey)
	p := "success"
	if err != nil {
		logger.Error("couldn't GET pointer from db", zap.Error(err))
	} else {	
		// WIP; i need to convert a custom type to string, 
		// will work on this later
		fmt.Println(res)	
		logger.Info("Got Pointer from db",
			zap.String("pointer", p),
		)
	}

	// Example List

	// listReq := proto.ListRequest{
	// 	// This pagination functionality doesn't work yet.
	// 	// The given arguments are placeholders.
	// 	StartingPathKey: []byte("test/pointer/path"),
	// 	Limit:           5,
	// 	APIKey:          []byte("abc123"),
	// }






	// listRes, err := client.List(ctx, &listReq)
	// if err != nil || status.Code(err) == codes.Internal {
	// 	logger.Error("failed to list file paths")
	// } else {
	// 	var stringList []string
	// 	for _, pathByte := range listRes.Paths {
	// 		stringList = append(stringList, string(pathByte))
	// 	}
	// 	logger.Debug("listed paths: " + strings.Join(stringList, ", "))
	// }

	// // Example Delete
	// // delete passes api creds
	// delReq := proto.DeleteRequest{
	// 	Path:   []byte("welcome/to/my/pointer/journey"),
	// 	APIKey: []byte("abc123"),
	// }
	// _, err = client.Delete(ctx, &delReq)
	// if err != nil || status.Code(err) == codes.Internal {
	// 	logger.Error("failed to delete: " + string(delReq.Path))
	// }
}
