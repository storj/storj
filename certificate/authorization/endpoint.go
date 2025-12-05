// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
)

// ErrEndpoint is the default error class for the authorization endpoint.
var ErrEndpoint = errs.Class("authorization endpoint")

// Endpoint provides a http endpoint for interacting with an authorization service.
type Endpoint struct {
	log      *zap.Logger
	service  *Service
	server   http.Server
	listener net.Listener
}

// NewEndpoint creates a authorization endpoint.
func NewEndpoint(log *zap.Logger, service *Service, listener net.Listener) *Endpoint {
	mux := http.NewServeMux()
	endpoint := &Endpoint{
		log:      log,
		listener: listener,
		service:  service,
		server: http.Server{
			Addr:    listener.Addr().String(),
			Handler: mux,
		},
	}

	mux.HandleFunc("/v1/authorizations/", endpoint.handleAuthorization)

	return endpoint
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
		err := endpoint.server.Serve(endpoint.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
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
	defer mon.Task()(&ctx)(&err)

	if httpReq.Method != http.MethodPut {
		msg := "unsupported HTTP method: " + httpReq.Method
		// NB: err set for `mon.Task` call.
		err = ErrEndpoint.New("%s", msg)
		http.Error(writer, msg, http.StatusMethodNotAllowed)
		return
	}

	userID := path.Base(httpReq.URL.Path)
	if userID == "authorizations" || userID == "" {
		msg := "missing user ID body"
		err = ErrEndpoint.New("%s", msg)
		http.Error(writer, msg, http.StatusUnprocessableEntity)
		return
	}

	token, err := endpoint.service.GetOrCreate(ctx, userID)
	if err != nil {
		msg := "error creating authorization"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		http.Error(writer, msg, http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusCreated)
	if _, err = writer.Write([]byte(token.String())); err != nil {
		msg := "error writing response"
		err = ErrEndpoint.Wrap(err)
		endpoint.log.Error(msg, zap.Error(err))
		// NB: status cannot be changed and the resource *was* created.
		http.Error(writer, msg, http.StatusCreated)
		return
	}
}
