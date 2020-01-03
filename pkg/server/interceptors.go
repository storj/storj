// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/common/identity"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/storj/storage"
)

func (server *Server) logOnErrorStreamInterceptor(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	err = handler(srv, ss)
	if err != nil {
		// no zap errors for canceled or wrong file downloads
		if storage.ErrKeyNotFound.Has(err) ||
			status.Code(err) == codes.Canceled ||
			status.Code(err) == codes.Unavailable ||
			err == io.EOF {
			return err
		}
		server.log.Error("gRPC stream error response", zap.Error(err))
	}
	return err
}

func (server *Server) logOnErrorUnaryInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	resp, err = handler(ctx, req)
	if err != nil {
		// no zap errors for wrong file downloads
		if status.Code(err) == codes.NotFound {
			return resp, err
		}
		server.log.Error("gRPC unary error response", zap.Error(err))
	}
	return resp, err
}

// CombineInterceptors combines two UnaryServerInterceptors so they act as one
// (because grpc only allows you to pass one in).
func CombineInterceptors(a, b grpc.UnaryServerInterceptor) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return a(ctx, req, info, func(actx context.Context, areq interface{}) (interface{}, error) {
			return b(actx, areq, info, func(bctx context.Context, breq interface{}) (interface{}, error) {
				return handler(bctx, breq)
			})
		})
	}
}

type nodeRequestLog struct {
	GRPCService string      `json:"grpc_service"`
	GRPCMethod  string      `json:"grpc_method"`
	PeerAddress string      `json:"peer_address"`
	PeerNodeID  string      `json:"peer_node_id"`
	Msg         interface{} `json:"msg"`
}

func prepareRequestLog(ctx context.Context, req, server interface{}, methodName string) ([]byte, error) {
	reqLog := nodeRequestLog{
		GRPCService: fmt.Sprintf("%T", server),
		GRPCMethod:  methodName,
		PeerAddress: "<no peer???>",
		Msg:         req,
	}
	if peer, err := rpcpeer.FromContext(ctx); err == nil {
		reqLog.PeerAddress = peer.Addr.String()
		if peerIdentity, err := identity.PeerIdentityFromPeer(peer); err == nil {
			reqLog.PeerNodeID = peerIdentity.ID.String()
		} else {
			reqLog.PeerNodeID = fmt.Sprintf("<no peer id: %v>", err)
		}
	}
	return json.Marshal(reqLog)
}

// UnaryMessageLoggingInterceptor creates a UnaryServerInterceptor which
// logs the full contents of incoming unary requests.
func UnaryMessageLoggingInterceptor(log *zap.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if jsonReq, err := prepareRequestLog(ctx, req, info.Server, info.FullMethod); err == nil {
			log.Info(string(jsonReq))
		} else {
			log.Error("Failed to marshal request to JSON.",
				zap.String("method", info.FullMethod), zap.Error(err),
			)
		}
		return handler(ctx, req)
	}
}

// StreamMessageLoggingInterceptor creates a StreamServerInterceptor which
// logs the full contents of incoming streaming requests.
func StreamMessageLoggingInterceptor(log *zap.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Are we even using any of these yet? I'm only guessing at how best to pass in things
		// so that they make sense.
		if jsonReq, err := prepareRequestLog(ss.Context(), srv, nil, info.FullMethod); err == nil {
			log.Info(string(jsonReq))
		} else {
			log.Error("Failed to marshal request to JSON.",
				zap.String("method", info.FullMethod), zap.Error(err),
			)
		}
		return handler(srv, ss)
	}
}
