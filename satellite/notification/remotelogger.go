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

func (log *RemoteLogger) Zap() *zap.Logger {
	return log.log
}

func (log *RemoteLogger) Debug(message string, fields ...zapcore.Field) {
	ctx := context.Background()

	address := log.getAddress(ctx, log.target)

	msg := &pb.NotificationMessage{
		NodeId:   log.target,
		Address:  address,
		Message:  []byte(message),
		Loglevel: pb.LogLevel_DEBUG,
	}

	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.log.Error("failed to process notification", zap.Error(err))
	}
}

func (log *RemoteLogger) Info(message string, fields ...zapcore.Field) {
	ctx := context.Background()

	address := log.getAddress(ctx, log.target)

	msg := &pb.NotificationMessage{
		NodeId:   log.target,
		Address:  address,
		Message:  []byte(message),
		Loglevel: pb.LogLevel_INFO,
	}

	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.log.Error("failed to process notification", zap.Error(err))
	}
}

func (log *RemoteLogger) Warn(message string, fields ...zapcore.Field) {
	ctx := context.Background()

	address := log.getAddress(ctx, log.target)

	msg := &pb.NotificationMessage{
		NodeId:   log.target,
		Address:  address,
		Message:  []byte(message),
		Loglevel: pb.LogLevel_WARN,
	}

	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.log.Error("failed to process notification", zap.Error(err))
	}
}

func (log *RemoteLogger) Error(message string, fields ...zapcore.Field) {
	ctx := context.Background()

	address := log.getAddress(ctx, log.target)

	msg := &pb.NotificationMessage{
		NodeId:   log.target,
		Address:  address,
		Message:  []byte(message),
		Loglevel: pb.LogLevel_ERROR,
	}

	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.log.Error("failed to process notification", zap.Error(err))
	}
}

func (log *RemoteLogger) getAddress(ctx context.Context, _ storj.NodeID) string {
	node, err := log.Service.overlay.Get(ctx, log.target)
	if err != nil {
		log.log.Error("failed to receive node info", zap.Error(err))
	}

	return node.GetAddress().GetAddress()
}
