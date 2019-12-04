// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"go.uber.org/zap"

	"storj.io/storj/pkg/sap"
	"storj.io/storj/pkg/storj"
)

var _ sap.Logger = (*Logger)(nil)

// Logger software that send specific data to nodes.
type Logger struct {
	zap.Logger
	*Service
}

// NewLogger returns new instance of Logger.
func NewLogger(log *zap.Logger, service *Service) *Logger {
	return &Logger{*log, service}
}

// Named adds a new path segment to the logger's name. Segments are joined by
// periods. By default, Loggers are unnamed.
func (log *Logger) Named(s string) sap.Logger {
	return &Logger{*log.log.Named(s), log.Service}
}

// Remote created remote logger to send notifications to specific Node by ID.
func (log *Logger) Remote(target storj.NodeID) sap.Logger {
	return &RemoteLogger{target: target, Service: log.Service, Logger: log.Logger}
}
