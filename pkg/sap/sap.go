// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sap

import (
	"go.uber.org/zap/zapcore"
	"storj.io/storj/pkg/storj"
)

type Logger interface {
	Named(s string) Logger
	Remote(target storj.NodeID) Logger

	Debug(message string, fields ...zapcore.Field)
	Info(message string, fields ...zapcore.Field)
	Warn(message string, fields ...zapcore.Field)
	Error(message string, fields ...zapcore.Field)
}
