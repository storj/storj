// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !windows

package process

import (
	"go.uber.org/zap/zapcore"
)

func levelDecorate(level zapcore.Level, message string) string {
	return levelColor[level] + message + colorReset
}

const (
	colorRed     = "\x1b[31m"
	colorGreen   = "\x1b[32m"
	colorBlue    = "\x1b[34m"
	colorMagenta = "\x1b[35m"
	colorReset   = "\x1b[0m"
)

var (
	levelColor = map[zapcore.Level]string{
		zapcore.DebugLevel:  colorGreen,
		zapcore.InfoLevel:   colorBlue,
		zapcore.WarnLevel:   colorMagenta,
		zapcore.ErrorLevel:  colorRed,
		zapcore.DPanicLevel: colorRed,
		zapcore.PanicLevel:  colorRed,
		zapcore.FatalLevel:  colorRed,
	}
)
