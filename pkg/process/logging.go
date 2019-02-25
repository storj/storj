// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"os"
	"runtime"

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
)

func newLogger() (*zap.Logger, error) {
	levelEncoder := zapcore.CapitalColorLevelEncoder
	if runtime.GOOS == "windows" {
		levelEncoder = zapcore.CapitalLevelEncoder
	}

	timeKey := "T"
	if os.Getenv("STORJ_LOG_NOTIME") != "" {
		// using environment variable STORJ_LOG_NOTIME to avoid additional flags
		timeKey = ""
	}

	return zap.Config{
		Level:             zap.NewAtomicLevelAt(*logLevel),
		Development:       *logDev,
		DisableCaller:     !*logCaller,
		DisableStacktrace: !*logStack,
		Encoding:          *logEncoding,
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        timeKey,
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    levelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{*logOutput},
		ErrorOutputPaths: []string{*logOutput},
	}.Build()
}
