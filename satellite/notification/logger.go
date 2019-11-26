// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"go.uber.org/zap"

	"storj.io/storj/pkg/sap"
	"storj.io/storj/pkg/storj"
)

type Logger struct {
	zap.Logger
	*Service
}

func NewLogger(log *zap.Logger, service *Service) *Logger {
	return &Logger{*log, service}
}

func (log *Logger) Named(s string) sap.Logger {
	return &Logger{*log.log.Named(s), log.Service}
}

func (log *Logger) Remote(target storj.NodeID) sap.Logger {
	return &RemoteLogger{target: target, Service: log.Service, Logger: log.Logger}
}

func (log *Logger) Zap() *zap.Logger {
	return log.log
}
