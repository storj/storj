// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package notification

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/storj/pkg/sap"
	"storj.io/storj/pkg/storj"

	"storj.io/storj/pkg/pb"
)

type RemoteLogger struct {
	zap.Logger
	target storj.NodeID
	*Service
}

func (log *RemoteLogger) Remote(target storj.NodeID) sap.Logger {
	return log
}

func (log *RemoteLogger) Named(s string) sap.Logger {
	return &RemoteLogger{*log.Logger.Named(s), log.target, log.Service}
}

func (log *RemoteLogger) ProcessDebug(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_DEBUG)
}

func (log *RemoteLogger) ProcessInfo(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_INFO)
}

func (log *RemoteLogger) ProcessWarn(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_WARN)
}

func (log *RemoteLogger) ProcessError(message string, fields ...zapcore.Field) {
	log.processNotification(message, pb.LogLevel_ERROR)
}

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

	err = log.Service.ProcessNotification(msg)
	if err != nil {
		log.log.Error("failed to process notification", zap.Error(err))
	}
}
