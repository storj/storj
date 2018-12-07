// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

type Server struct {
	logger  *zap.Logger
	metrics *monkit.Registry
}

func NewServer(log *zap.Logger) *Server {
	return &Server{
		logger:  log,
		metrics: monkit.Default,
	}
}
