// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/sap"
	"storj.io/storj/pkg/storj"
)

var _ sap.Logger = (*RemoteLogger)(nil)

// RemoteLogger logger that sends log to specific storageNode.
type RemoteLogger struct {
	zap.Logger
	target storj.NodeID
	*Service
}

func (log *RemoteLogger) Remote(target storj.NodeID) sap.Logger {
	return log
}

// Named adds a new path segment to the logger's name. Segments are joined by
// periods. By default, Loggers are unnamed.
func (log *RemoteLogger) Named(s string) sap.Logger {
	return &RemoteLogger{*log.Logger.Named(s), log.target, log.Service}
}

// Debug sends a DebugLevel message to specified set of nodes. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *RemoteLogger) Debug(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_DEBUG)
}

// Info sends a DebugLevel message to specified set of nodes. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *RemoteLogger) Info(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_INFO)
}

// warn sends a DebugLevel message to specified set of nodes. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *RemoteLogger) Warn(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_WARN)
}

// Error sends a DebugLevel message to specified set of nodes. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func (log *RemoteLogger) Error(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_ERROR)
}

// ProcessNotification sends message to the specified set of nodes (ids)
func (log *RemoteLogger) processNotification(message string, level pb.LogLevel) {
	ctx := context.Background()
	node, err := log.Service.overlay.Get(ctx, log.target)
	if err != nil {
		log.log.Error("failed to receive node info", zap.Error(err))
	}

	address := node.GetAddress().GetAddress()

	msg := &pb.NotificationMessage{
		NodeId:   log.target,
		Address:  address,
		Message:  []byte(message),
		Loglevel: level,
	}

	err = log.Service.ProcessNotification(ctx, msg)
	if err != nil {
		log.log.Error("failed to process notification", zap.Error(Error.Wrap(err)))
	}
}
