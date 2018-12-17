// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Error is a process error class
	Error = errs.Class("process error")

	logLevel    = zap.LevelFlag("log.level", zapcore.WarnLevel, "the minimum log level to log")
	logDev      = flag.Bool("log.development", false, "if true, set logging to development mode")
	logCaller   = flag.Bool("log.caller", false, "if true, log function filename and line number")
	logStack    = flag.Bool("log.stack", false, "if true, log stack traces")
	logEncoding = flag.String("log.encoding", "console", "configures log encoding. can either be 'console' or 'json'")
	logOutput   = flag.String("log.output", "stderr", "can be stdout, stderr, or a filename")
	logPrefix   = flag.String("log.prefix", "", "log lines using prefix")
)

func newLogger() (*zap.Logger, error) {
	logger, err := zap.Config{
		Level:             zap.NewAtomicLevelAt(*logLevel),
		Development:       *logDev,
		DisableCaller:     !*logCaller,
		DisableStacktrace: !*logStack,
		Encoding:          *logEncoding,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalColorLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{*logOutput},
		ErrorOutputPaths: []string{*logOutput},
	}.Build()

	if err != nil {
		return logger, err
	}

	return logger.Named(*logPrefix), err
}
