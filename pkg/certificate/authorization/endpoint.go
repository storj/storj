// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pb"
)

// ErrEndpoint is the default error class for the authorization endpoint.
var ErrEndpoint = errs.Class("authorization endpoint error")

// Endpoint implements pb.AuthorizationsServer
type Endpoint struct {
	log      *zap.Logger
	db       *DB
	server   http.Server
	listener net.Listener
}

// NewEndpoint creates a new authorization gRPC server.
func NewEndpoint(log *zap.Logger, db *DB, listener net.Listener) *Endpoint {
	mux := http.NewServeMux()
	endpoint := &Endpoint{
		log:      log,
		db:       db,
		listener: listener,
		server: http.Server{
			Handler: mux,
		},
	}

	mux.HandleFunc("/v1/authorization/create", endpoint.httpCreate)

	return endpoint
}

// Create creates an authorization from the given authorization request.
func (endpoint *Endpoint) Create(ctx context.Context, req *pb.AuthorizationRequest) (_ *pb.AuthorizationResponse, err error) {
	mon.Task()(&ctx, req.UserId)(&err)

	existingGroup, err := endpoint.db.Get(ctx, req.UserId)
	if err != nil {
		msg := "error getting authorizations"
		err := ErrEndpoint.New(msg)
		endpoint.log.Error(msg, zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if len(existingGroup) > 0 {
		authorization := existingGroup[0]
		return &pb.AuthorizationResponse{
			Token: authorization.Token.String(),
		}, nil
	}

	createdGroup, err := endpoint.db.Create(ctx, req.UserId, 1)
	if err != nil {
		msg := "error creating authorization"
		err := ErrEndpoint.New(msg)
		endpoint.log.Error(msg, zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	groupLen := len(createdGroup)
	if groupLen != 1 {
		clientMsg := "error creating authorization"
		internalMsg := clientMsg + fmt.Sprintf("; expected 1, got %d", groupLen)

		endpoint.log.Error(internalMsg)
		return nil, status.Error(codes.Internal, ErrEndpoint.New(clientMsg).Error())
	}

	authorization := createdGroup[0]

	return &pb.AuthorizationResponse{
		Token: authorization.Token.String(),
	}, nil
}

// Run starts the endpoint HTTP server and waits for the context to be
// cancelled or for `Close` to be called.
func (endpoint *Endpoint) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return endpoint.server.Shutdown(context.Background())
	})
	group.Go(func() error {
		defer cancel()
		return endpoint.server.Serve(endpoint.listener)
	})

	return group.Wait()
}

// Close closes the endpoint HTTP server.
func (endpoint *Endpoint) Close() error {
	return endpoint.server.Close()
}

func (endpoint *Endpoint) httpCreate(writer http.ResponseWriter, httpReq *http.Request) {
	var err error
	ctx := context.Background()
	mon.Task()(&ctx)(&err)

	if httpReq.Method != http.MethodPost {
		msg := fmt.Sprintf("unsupported HTTP method: %s", httpReq.Method)
		err = ErrEndpoint.New(msg)
		endpoint.log.Error(msg, zap.Error(ErrEndpoint.New(msg)))
		http.Error(writer, msg, http.StatusBadRequest)
		return
	}

	userID, err := ioutil.ReadAll(httpReq.Body)
	if bytes.Equal([]byte{}, userID) {
		msg := "missing user ID body"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusBadRequest)
		return
	}
	if err != nil {
		msg := "error reading body"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}

	authorizationReq := &pb.AuthorizationRequest{
		UserId: string(userID),
	}

	authorizationRes, err := endpoint.Create(ctx, authorizationReq)
	if err != nil {
		msg := "error creating authorization"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
	if _, err = writer.Write([]byte(authorizationRes.Token)); err != nil {
		msg := "error writing response"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}
}
