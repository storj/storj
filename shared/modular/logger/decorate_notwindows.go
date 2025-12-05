// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !windows

package logger

import (
	"go.uber.org/zap/zapcore"
)

const (
	colorReset   = "\x1b[0m"
	colorRed     = "\x1b[31m"
	colorGreen   = "\x1b[32m"
	colorBlue    = "\x1b[34m"
	colorMagenta = "\x1b[35m"
)

func levelDecorate(level zapcore.Level, message string) string {
	return levelColor[level] + message + colorReset
}

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
