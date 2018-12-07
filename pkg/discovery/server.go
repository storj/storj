// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

// Server struct that loads on logging and metrics
type Server struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

// NewServer returns a server
func NewServer(log *zap.Logger) *Server {
	return &Server{
		logger:  log,
		metrics: monkit.Default,
	}
}
