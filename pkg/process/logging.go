// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"net/url"
	"os"
	"runtime"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"storj.io/storj/pkg/cfgstruct"
)

var (
	// Error is a process error class
	Error = errs.Class("process error")

	logLevel = zap.LevelFlag("log.level", func() zapcore.Level {
		if isDev() {
			return zapcore.DebugLevel
		}
		return zapcore.InfoLevel
	}(), "the minimum log level to log")
	logDev      = flag.Bool("log.development", isDev(), "if true, set logging to development mode")
	logCaller   = flag.Bool("log.caller", isDev(), "if true, log function filename and line number")
	logStack    = flag.Bool("log.stack", isDev(), "if true, log stack traces")
	logEncoding = flag.String("log.encoding", "console", "configures log encoding. can either be 'console' or 'json'")
	logOutput   = flag.String("log.output", "stderr", "can be stdout, stderr, or a filename")
)

func init() {
	winFileSink := func(u *url.URL) (zap.Sink, error) {
		// Remove leading slash left by url.Parse()
		return os.OpenFile(u.Path[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	}
	err := zap.RegisterSink("winfile", winFileSink)
	if err != nil {
		panic("Unable to register winfile sink: " + err.Error())
	}
}

func isDev() bool { return cfgstruct.DefaultsType() != "release" }

// NewLogger creates new logger configured by the process flags.
func NewLogger() (*zap.Logger, error) {
	return NewLoggerWithOutputPaths(*logOutput)
}

// NewLoggerWithOutputPaths is the same as NewLogger, but overrides the log output paths.
func NewLoggerWithOutputPaths(outputPaths ...string) (*zap.Logger, error) {
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
		OutputPaths:      outputPaths,
		ErrorOutputPaths: outputPaths,
	}.Build()
}
