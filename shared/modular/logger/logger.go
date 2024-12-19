// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package logger

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"

	"storj.io/common/process/gcloudlogging"
	"storj.io/storj/shared/mud"
)

// Config is the configuration for the logging.
type Config struct {
	Level       string `help:"the minimum log level to log" default:"info"`
	Development bool   `help:"if true, set logging to development mode" default:"false"`
	Caller      bool   `help:"if true, log function filename and line number" default:"false"`
	Stack       bool   `help:"if true, log stack traces" default:"false"`
	Encoding    string `help:"configures log encoding. can either be 'console', 'json', 'pretty', or 'gcloudlogging'." default:"console"`
	Output      string `help:"can be stdout, stderr, or a filename" default:"stderr"`

	CustomLevel string `help:"custom level overrides for specific loggers in the format NAME1=ERROR,NAME2=WARN,... Only level increment is supported, and only for selected loggers!" default:""`
}

var (
	defaultLogEncoderConfig = map[string]zapcore.EncoderConfig{
		"gcloudlogging": gcloudlogging.NewEncoderConfig(),
	}
)

// NewZapConfig creates a new ZapConfig.
func NewZapConfig(config Config) (*zap.Config, error) {
	{
		err := zap.RegisterEncoder("prettymud", func(encoderConfig zapcore.EncoderConfig) (zapcore.Encoder, error) {
			return newPrettyEncoder(encoderConfig, config.Development), nil
		})
		if err != nil {
			panic("Unable to register pretty encoder: " + err.Error())
		}
	}

	timeKey := "T"
	if os.Getenv("STORJ_LOG_NOTIME") != "" {
		// using environment variable STORJ_LOG_NOTIME to avoid additional flags
		timeKey = ""
	}

	atomicLevel, err := zap.ParseAtomicLevel(config.Level)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	var encoderConfig zapcore.EncoderConfig

	if config.Encoding == "pretty" {
		// we need the version which has access to our dynamic configuration values.
		// all others are registered with init()
		config.Encoding = "prettymud"
	}
	if v, ok := defaultLogEncoderConfig[config.Encoding]; ok {
		encoderConfig = v
	} else { // fallback to default config
		encoderConfig = zapcore.EncoderConfig{
			TimeKey:        timeKey,
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "C",
			MessageKey:     "M",
			StacktraceKey:  "S",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.RFC3339TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}
	}

	cfg := &zap.Config{
		Level:             atomicLevel,
		Development:       config.Development,
		DisableCaller:     !config.Caller,
		DisableStacktrace: !config.Stack,
		Encoding:          config.Encoding,
		EncoderConfig:     encoderConfig,
		OutputPaths:       []string{config.Output},
		ErrorOutputPaths:  []string{config.Output},
	}

	return cfg, nil
}

// RootLogger is a wrapper type to register the root logger.
type RootLogger struct {
	*zap.Logger
}

// NewRootLogger creates the actual logger.
func NewRootLogger(config *zap.Config) (RootLogger, error) {
	build, err := config.Build()
	return RootLogger{
		Logger: build,
	}, err
}

// NewLogger creates a new logger.
func NewLogger(cfg Config, logger RootLogger) mud.Injector[*zap.Logger] {
	return func(ball *mud.Ball, rt reflect.Type) *zap.Logger {
		name := strings.ToLower(rt.String())
		name = strings.ReplaceAll(name, "*", "")
		name = strings.ReplaceAll(name, ".", ":")
		return NamedLog(logger.Logger, name, cfg.CustomLevel)
	}
}

// NamedLog creates a new named logger, supporting custom log levels.
func NamedLog(base *zap.Logger, name string, customLevel string) *zap.Logger {
	child := base.Named(name)
	for _, customization := range strings.Split(customLevel, ",") {
		customization = strings.TrimSpace(customization)
		if len(customization) == 0 {
			continue
		}
		parts := strings.SplitN(customization, "=", 2)
		if len(parts) != 2 {
			child.Warn("Invalid log level override. Use name=LEVEL format.")
			continue
		}
		if parts[0] == name {
			var level zapcore.Level
			err := level.UnmarshalText([]byte(parts[1]))
			if err != nil {
				child.Warn("Invalid log level override", zap.String("level", parts[1]))
			} else {
				child = child.WithOptions(zap.IncreaseLevel(level))
			}
			break
		}
	}
	return child
}

type prettyEncoder struct {
	*zapcore.MapObjectEncoder
	config      zapcore.EncoderConfig
	pool        buffer.Pool
	development bool
}

func newPrettyEncoder(config zapcore.EncoderConfig, development bool) *prettyEncoder {
	return &prettyEncoder{
		MapObjectEncoder: zapcore.NewMapObjectEncoder(),
		config:           config,
		pool:             buffer.NewPool(),
		development:      development,
	}
}

func (p *prettyEncoder) Clone() zapcore.Encoder {
	rv := newPrettyEncoder(p.config, false)
	for key, val := range p.MapObjectEncoder.Fields {
		rv.MapObjectEncoder.Fields[key] = val
	}
	return rv
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (p *prettyEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	b := p.pool.Get()

	_, _ = fmt.Fprintf(b, "%s\t%s\t%s\n",
		entry.Time.Format("15:04:05.000"),
		levelDecorate(entry.Level, entry.Level.CapitalString()),
		entry.Message)

	for _, field := range fields {
		m := zapcore.NewMapObjectEncoder()
		field.AddTo(m)
		for _, key := range sortedKeys(m.Fields) {
			if key == "errorVerbose" && !p.development {
				continue
			}
			_, _ = fmt.Fprintf(b, "\t%s: %s\n",
				key,
				strings.ReplaceAll(fmt.Sprint(m.Fields[key]), "\n", "\n\t"))
		}
	}

	return b, nil
}
