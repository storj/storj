// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package logger

import (
	"go.uber.org/zap/zapcore"
)

func levelDecorate(level zapcore.Level, message string) string {
	return message
}
