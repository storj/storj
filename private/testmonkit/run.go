// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

// Package testmonkit allows attaching monkit monitoring for testing.
//
// It allows to set an environment variable to get a trace per test.
//
//	STORJ_TEST_MONKIT=svg
//	STORJ_TEST_MONKIT=json
//
// By default, it saves the output the same folder as the test. However, if you wish
// to specify a separate folder, you can specify an absolute directory:
//
//	STORJ_TEST_MONKIT=json,svg,dir=/home/user/debug/trace
//
// Note, due to how go tests work, it's not possible to specify a relative directory.
package testmonkit

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/collect"
	"github.com/spacemonkeygo/monkit/v3/present"
)

var mon = monkit.Package()

// Config defines configuration for monkit test output.
type Config struct {
	Disabled bool
	Dir      string
	Outputs  []string
}

// Run attaches monkit tracing to the ctx where configuration is taken from STORJ_TEST_MONKIT.
func Run(ctx context.Context, tb testing.TB, fn func(ctx context.Context)) {
	RunWith(ctx, tb, EnvConfig(tb), fn)
}

// RunWith attaches monkit to the ctx with custom configuration.
func RunWith(parentCtx context.Context, tb testing.TB, cfg Config, fn func(ctx context.Context)) {
	if cfg.Disabled {
		fn(parentCtx)
		return
	}

	done := mon.Task()(&parentCtx)
	spans := collect.CollectSpans(parentCtx, fn)
	done(nil)

	baseName := sanitizeFileName(tb.Name())

	for _, outputType := range cfg.Outputs {
		var data bytes.Buffer

		var err error
		switch outputType {
		case "svg":
			err = present.SpansToSVG(&data, spans)
		case "json":
			err = present.SpansToJSON(&data, spans)
		}
		if err != nil {
			tb.Error(err)
		}

		path := filepath.Join(cfg.Dir, baseName+".test."+outputType)
		err = os.WriteFile(path, data.Bytes(), 0644)
		if err != nil {
			tb.Errorf("failed to write %q: %v", path, err)
		}
	}
}

var supportedOutputs = map[string]bool{
	"svg":  true,
	"json": true,
}

// EnvConfig loads test monkit configuration from STORJ_TEST_MONKIT environment variable.
func EnvConfig(tb testing.TB) Config {
	value := os.Getenv("STORJ_TEST_MONKIT")
	if value == "" {
		return Config{Disabled: true}
	}

	cfg := Config{}

	for _, tag := range strings.Split(value, ",") {
		tokens := strings.SplitN(tag, "=", 2)
		if len(tokens) <= 1 {
			tag = strings.TrimSpace(tag)
			if !supportedOutputs[tag] {
				tb.Errorf("testmonkit: unknown output type %q", tag)
				continue
			}
			cfg.Outputs = append(cfg.Outputs, tag)
			continue
		}

		key, value := strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1])
		switch key {
		case "dir":
			cfg.Dir = value
		case "type":
			cfg.Outputs = append(cfg.Outputs, strings.TrimSpace(tag))
		default:
			tb.Errorf("testmonkit: unhandled key=%q value=%q", key, value)
		}
	}

	return cfg
}

func sanitizeFileName(s string) string {
	var b strings.Builder
	for _, x := range s {
		switch {
		case 'a' <= x && x <= 'z':
			b.WriteRune(x)
		case 'A' <= x && x <= 'Z':
			b.WriteRune(x)
		case '0' <= x && x <= '9':
			b.WriteRune(x)
		}
	}
	return b.String()
}
