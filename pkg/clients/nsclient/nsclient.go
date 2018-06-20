// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nsclient

import (
	"context"

	"google.golang.org/grpc"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"



	"storj.io/storj/pkg/netstate"
	pb "storj.io/storj/protos/netstate"
)

func NewNSClient(address string conn *grpc.ClientConn) (NSClient, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		logger.Error("Failed to dial: ", zap.Error(err))
		return err
	}
	return &NSClient{
			netstateclient: conn,
		}
	}, nil
}

func NetStateClient struct {
//	Path netstate.Path
//	APIKey string
	netstateClient pb.NetStateClient
}

type NSClient interface 
	Put(ctx context.Context, path netstate.Path, pointer *pb.Pointer) error
	Get(ctx context.Context, path netstate.Path) (*pb.Pointer, error)
	Delete(ctx context.Context, path netstate.Path) error
	List(ctx context.Context, startingPath, endingPath netstate.Path) (
		paths []dtypes.Path, truncated bool, err error)
}


func (ns *NetStateClient ) Put(ctx context.Context, path netstate.Path, pointer *pb.Pointer, APIKey []byte) error {
	resp, err := ns.netStateClient.Put(ctx, path, pointer, APIKey)
	if err != nil {
		logger.Error("Failed to make a PUT request ", zap.Error(err))
		return err
	}
	return status.Errorf(codes.Internal, err.Error())
}



// func (ns *NetStateClient ) Get()
// func (ns *NetStateClient ) List()
// func (ns *NetStateClient ) Delete()