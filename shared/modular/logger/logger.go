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
	"go.opentelemetry.io/contrib/bridges/otelzap"
	"go.opentelemetry.io/otel/sdk/log"
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
	UseOtelOnly bool   `help:"if true, only forward logs to the OpenTelemetry logger provider and skip the standard console/file output" default:"false"`

	CustomLevel string `help:"custom level overrides for specific loggers in the format NAME1=ERROR,NAME2=WARN,... Only level increment is supported, and only for selected loggers!" default:""`
}

var (
	defaultLogEncoderConfig = map[string]zapcore.EncoderConfig{
		"gcloudlogging": gcloudlogging.NewEncoderConfig(),
	}
)

// NewZapConfig creates a new ZapConfig.
func NewZapConfig(config Config) (*zap.Config, error) {

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
func NewRootLogger(cfg Config, config *zap.Config, provider *log.LoggerProvider) (RootLogger, error) {
	otelCore := otelzap.NewCore("storj", otelzap.WithLoggerProvider(provider))

	opts := []zap.Option{}

	if config.Development {
		opts = append(opts, zap.Development())
	}

	if !config.DisableCaller {
		opts = append(opts, zap.AddCaller())
	}

	stackLevel := config.Level.Level()
	if config.Development {
		stackLevel = zap.WarnLevel
	}
	if !config.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(stackLevel))
	}

	// When UseOtelOnly is set, forward everything through the OpenTelemetry core and
	// skip the standard encoder/output core entirely.
	if cfg.UseOtelOnly {
		root := zap.New(otelCore, opts...)
		return RootLogger{
			Logger: root,
		}, nil
	}

	var encoder zapcore.Encoder
	switch config.Encoding {
	case "json":
		encoder = zapcore.NewJSONEncoder(config.EncoderConfig)
	case "console":
		encoder = zapcore.NewConsoleEncoder(config.EncoderConfig)
	case "prettymud":
		encoder = newPrettyEncoder(config.EncoderConfig, config.Development)
	case "gcloudlogging":
		encoder = gcloudlogging.NewEncoder(config.EncoderConfig)
	default:
		return RootLogger{}, errs.New("unsupported log encoding: %q", config.Encoding)
	}

	outsync, _, err := zap.Open(config.OutputPaths...)
	if err != nil {
		return RootLogger{}, errs.Wrap(err)
	}

	errsync, _, err := zap.Open(config.ErrorOutputPaths...)
	if err != nil {
		return RootLogger{}, errs.Wrap(err)
	}

	opts = append(opts, zap.ErrorOutput(errsync))

	core := zapcore.NewCore(encoder, outsync, config.Level)
	combinedCore := zapcore.NewTee(core, otelCore)

	root := zap.New(combinedCore, opts...)
	return RootLogger{
		Logger: root,
	}, nil
}

// Close flushes any buffered log entries of the root logger.
func Close(r *RootLogger) error {
	if r.Logger != nil {
		return r.Logger.Sync()
	}
	return nil
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
