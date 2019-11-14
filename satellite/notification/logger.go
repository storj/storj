package notification

import (
	"go.uber.org/zap/zapcore"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/sap"
	"storj.io/storj/pkg/storj"
)

type Logger struct {
	sap.Logger
	*Service
}

func (log *Logger) Remote(target storj.NodeID) sap.Logger {
	return &RemoteLogger{log.Logger, log.Service, target}
}

func (log *Logger) Debug(message string, fields ...zapcore.Field) {
	msg := &pb.NotificationMessage{Message: []byte(message)}
	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.Remote(msg.NodeId)
	}
}

func (log *Logger) Info(message string, fields ...zapcore.Field) {
	msg := &pb.NotificationMessage{Message: []byte(message)}
	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.Remote(msg.NodeId)
	}
}

func (log *Logger) Warn(message string, fields ...zapcore.Field) {
	msg := &pb.NotificationMessage{Message: []byte(message)}
	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.Remote(msg.NodeId)
	}
}

func (log *Logger) Error(message string, fields ...zapcore.Field) {
	msg := &pb.NotificationMessage{Message: []byte(message)}
	err := log.Service.ProcessNotification(msg)
	if err != nil {
		log.Remote(msg.NodeId)
	}
}

type RemoteLogger struct {
	sap.Logger
	*Service
	Target storj.NodeID
}

func (log *RemoteLogger) Remote(target storj.NodeID) sap.Logger {
	return log
}

func (log *RemoteLogger) Named(s string) sap.Logger {
	return log.Named(s)
}
