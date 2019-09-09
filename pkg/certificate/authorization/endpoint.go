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

	"storj.io/storj/pkg/pb"
)

// ErrEndpoint is the default error class for the authorization endpoint.
var ErrEndpoint = errs.Class("authorization endpoint error")

// Endpoint implements pb.AuthorizationsServer.
type Endpoint struct {
	log      *zap.Logger
	db       *DB
	service  *Service
	server   http.Server
	listener net.Listener
}

// NewEndpoint creates a new http proxy for an authorization service.
func NewEndpoint(log *zap.Logger, db *DB, listener net.Listener) *Endpoint {
	service := NewService(log, db)
	mux := http.NewServeMux()
	endpoint := &Endpoint{
		log:      log,
		db:       db,
		listener: listener,
		service:  service,
		server: http.Server{
			Addr:    listener.Addr().String(),
			Handler: mux,
		},
	}

	mux.HandleFunc("/v1/authorization", endpoint.handleAuthorization)

	return endpoint
}

// Create creates an authorization from the given authorization request.
func (endpoint *Endpoint) Create(ctx context.Context, req *pb.AuthorizationRequest) (_ *pb.AuthorizationResponse, err error) {
	mon.Task()(&ctx, req.UserId)(&err)
	token, err := endpoint.service.GetOrCreate(ctx, req.UserId)

	return &pb.AuthorizationResponse{
		Token: token.String(),
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

func (endpoint *Endpoint) handleAuthorization(writer http.ResponseWriter, httpReq *http.Request) {
	var err error
	ctx := httpReq.Context()
	mon.Task()(&ctx)(&err)

	if httpReq.Method != http.MethodPut {
		msg := fmt.Sprintf("unsupported HTTP method: %s", httpReq.Method)
		err = ErrEndpoint.New(msg)
		http.Error(writer, msg, http.StatusMethodNotAllowed)
		return
	}

	userID, err := ioutil.ReadAll(httpReq.Body)
	if err != nil {
		msg := "error reading body"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}
	if bytes.Equal([]byte{}, userID) {
		msg := "missing user ID body"
		http.Error(writer, msg, http.StatusUnprocessableEntity)
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
		// NB: status cannot be changed and the resource *was* created.
		http.Error(writer, msg, http.StatusCreated)
		return
	}
}
