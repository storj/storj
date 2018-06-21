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

	nsclient, err := client.NewNetStateClient(port)
	if err != nil {
		fmt.Println("error page !!!!!")
		logger.Error("Failed to dial: ", zap.Error(err))
	}

	client := proto.NewPointerDBClient(conn)

	logger.Debug(fmt.Sprintf("client dialed port %s", port))


	// conn, err := grpc.Dial(port, grpc.WithInsecure())
	// if err != nil {
	// 	logger.Error("Failed to dial: ", zap.Error(err))
	// }

	// client := proto.NewNetStateClient(conn)

	// logger.Debug(fmt.Sprintf("client dialed port %s", port))

	ctx := context.Background()






	// Example pointer paths to put
	pr1 := proto.PutRequest{
		Path: []byte("test/path/1"),
		Pointer: &proto.Pointer{
			Type:          proto.Pointer_INLINE,
			InlineSegment: []byte("inline1"),
		},
		APIKey: apiKey,
	}
	pr2 := proto.PutRequest{
		Path: []byte("test/path/2"),
		Pointer: &proto.Pointer{
			Type:          proto.Pointer_INLINE,
			InlineSegment: []byte("inline2"),
		},
		APIKey: apiKey,
	}
	pr3 := proto.PutRequest{
		Path: []byte("test/path/3"),
		Pointer: &proto.Pointer{
			Type:          proto.Pointer_INLINE,
			InlineSegment: []byte("inline3"),
		},
		APIKey: apiKey,
	}
	// rps is an example slice of RemotePieces, which is passed into
	// this example Pointer of type REMOTE.
	var rps []*proto.RemotePiece
	rps = append(rps, &proto.RemotePiece{
		PieceNum: int64(1),
		NodeId:   "testId",
	})
	pr4 := proto.PutRequest{
		Path: []byte("test/path/4"),
		Pointer: &proto.Pointer{
			Type: proto.Pointer_REMOTE,
			Remote: &proto.RemoteSegment{
				Redundancy: &proto.RedundancyScheme{
					Type:             proto.RedundancyScheme_RS,
					MinReq:           int64(1),
					Total:            int64(3),
					RepairThreshold:  int64(2),
					SuccessThreshold: int64(3),
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
		},
		APIKey: apiKey,
	}

	// Example Puts
	// puts passes api creds
	_, err = nsclient.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		logger.Error("failed to put", zap.Error(err))
	}
	





	
	// _, err = client.Put(ctx, &pr2)
	// if err != nil || status.Code(err) == codes.Internal {
	// 	logger.Error("failed to put", zap.Error(err))
	// }
	// _, err = client.Put(ctx, &pr3)
	// if err != nil || status.Code(err) == codes.Internal {
	// 	logger.Error("failed to put", zap.Error(err))
	// }




	// Example Get
	// get passes api creds
	// getReq := proto.GetRequest{
	// 	Path:   []byte("so/many/pointers"),
	// 	APIKey: []byte("abc123"),
	// }
	// getRes, err := client.Get(ctx, &getReq)
	// if err != nil || status.Code(err) == codes.Internal {
	// 	logger.Error("failed to get", zap.Error(err))
	// } else {
	// 	pointer := string(getRes.Pointer)
	// 	logger.Debug("get response: " + pointer)
	// }

	// // Example List
	// // list passes api creds
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
