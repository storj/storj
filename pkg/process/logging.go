// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
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
	logEncoding = flag.String("log.encoding", "pretty", "configures log encoding. can either be 'console', 'pretty', or 'json'")
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

	err = zap.RegisterEncoder("pretty", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return newPrettyEncoder(config), nil
	})
	if err != nil {
		panic("Unable to register pretty encoder: " + err.Error())
	}
}

func isDev() bool { return cfgstruct.DefaultsType() != "release" }

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

type prettyEncoder struct {
	*zapcore.MapObjectEncoder
	config zapcore.EncoderConfig
	pool   buffer.Pool
}

func newPrettyEncoder(config zapcore.EncoderConfig) *prettyEncoder {
	return &prettyEncoder{
		MapObjectEncoder: zapcore.NewMapObjectEncoder(),
		config:           config,
		pool:             buffer.NewPool(),
	}
}

func (p *prettyEncoder) Clone() zapcore.Encoder {
	rv := newPrettyEncoder(p.config)
	for key, val := range p.MapObjectEncoder.Fields {
		rv.MapObjectEncoder.Fields[key] = val
	}
	return rv
}

func (p *prettyEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	b := p.pool.Get()

	fmt.Fprintf(b, "%s\t%s\t%s\n",
		entry.Time.Format("15:04:05.000"),
		levelDecorate(entry.Level, entry.Level.CapitalString()),
		entry.Message)
	for _, field := range fields {
		m := zapcore.NewMapObjectEncoder()
		field.AddTo(m)
		keys := make([]string, 0, len(m.Fields))
		for key := range m.Fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if key == "errorVerbose" && !*logDev {
				continue
			}
			fmt.Fprintf(b, "\t%s: %s\n", key, strings.Replace(fmt.Sprint(m.Fields[key]), "\n", "\n\t", -1))
		}
	}
	return b, nil
}
