// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

// Server struct that loads on logging and metrics
type Server struct {
	log     *zap.Logger
	metrics *monkit.Registry
}

// NewServer returns a server
func NewServer(l *zap.Logger) *Server {
	return &Server{
		log:     l,
		metrics: monkit.Default,
	}
}
