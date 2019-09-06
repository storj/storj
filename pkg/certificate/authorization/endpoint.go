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

var ErrEndpoint = errs.Class("authorization endpoint error")

type Endpoint struct {
	log      *zap.Logger
	db       *DB
	server   http.Server
	listener net.Listener
}

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

func (endpoint *Endpoint) Create(ctx context.Context, req *pb.AuthorizationRequest) (_ *pb.AuthorizationResponse, err error) {
	mon.Task()(&ctx, req.UserId)(&err)

	group, err := endpoint.db.Create(ctx, req.UserId, 1)
	if err != nil {
		msg := "error creating authorization"
		err := ErrEndpoint.New(msg)
		endpoint.log.Error(msg, zap.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	groupLen := len(group)
	if groupLen != 1 {
		clientMsg := "error creating authorization"
		internalMsg := clientMsg + fmt.Sprintf("; expected 1, got %d", groupLen)

		endpoint.log.Error(internalMsg)
		return nil, status.Error(codes.Internal, ErrEndpoint.New(clientMsg).Error())
	}

	authorization := group[0]

	return &pb.AuthorizationResponse{
		Token: authorization.Token.String(),
	}, nil
}

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

func (endpoint *Endpoint) Close() error {
	return endpoint.server.Close()
}

func (endpoint *Endpoint) httpCreate(writer http.ResponseWriter, httpReq *http.Request) {
	ctx := context.Background()

	if httpReq.Method != http.MethodPost {
		msg := fmt.Sprintf("unsupported HTTP method: %s", httpReq.Method)
		endpoint.log.Error(msg, zap.Error(ErrEndpoint.New(msg)))
		http.Error(writer, msg, http.StatusBadRequest)
		return
	}

	fmt.Println("no error!")
	userID, err := ioutil.ReadAll(httpReq.Body)
	if err != nil || bytes.Equal([]byte{}, userID) {
		msg := "error reading body"
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusBadRequest)
		return
	}

	authorizationReq := &pb.AuthorizationRequest{
		UserId: string(userID),
	}

	authorizationRes, err := endpoint.Create(ctx, authorizationReq)
	if err != nil {
		msg := "error creating authorization"
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
	if _, err = writer.Write([]byte(authorizationRes.Token)); err != nil {
		msg := "error writing response"
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}
}
