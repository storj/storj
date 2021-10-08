// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"testing"

	"github.com/spacemonkeygo/monkit/v3/collect"
	"github.com/spacemonkeygo/monkit/v3/present"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/private/dbutil/pgtest"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/uplink"
)

// Run runs testplanet in multiple configurations.
func Run(t *testing.T, config Config, test func(t *testing.T, ctx *testcontext.Context, planet *Planet)) {
	databases := satellitedbtest.Databases()
	if len(databases) == 0 {
		t.Fatal("Databases flag missing, set at least one:\n" +
			"-postgres-test-db=" + pgtest.DefaultPostgres + "\n" +
			"-cockroach-test-db=" + pgtest.DefaultCockroach)
	}

	for _, satelliteDB := range databases {
		satelliteDB := satelliteDB
		t.Run(satelliteDB.Name, func(t *testing.T) {
			parallel := !config.NonParallel
			if parallel {
				t.Parallel()
			}

			if satelliteDB.MasterDB.URL == "" {
				t.Skipf("Database %s connection string not provided. %s", satelliteDB.MasterDB.Name, satelliteDB.MasterDB.Message)
			}
			planetConfig := config
			if planetConfig.Name == "" {
				planetConfig.Name = t.Name()
			}

			log := newLogger(t)

			startPlanetAndTest := func(parent context.Context) {
				ctx := testcontext.NewWithContext(parent, t)
				defer ctx.Cleanup()

				pprof.Do(ctx, pprof.Labels("planet", planetConfig.Name), func(namedctx context.Context) {
					planet, err := NewCustom(namedctx, log, planetConfig, satelliteDB)
					if err != nil {
						t.Fatalf("%+v", err)
					}
					defer ctx.Check(planet.Shutdown)

					planet.Start(namedctx)

					provisionUplinks(namedctx, t, planet)

					test(t, ctx, planet)
				})
			}

			monkitConfig := os.Getenv("STORJ_TEST_MONKIT")
			if monkitConfig == "" {
				startPlanetAndTest(context.Background())
			} else {
				flags := parseMonkitFlags(monkitConfig)
				outDir := flags["dir"]
				if outDir != "" {
					if !filepath.IsAbs(outDir) {
						t.Fatalf("testplanet-monkit: dir must be an absolute path, but was %q", outDir)
					}
				}
				outType := flags["type"]

				rootctx := context.Background()

				done := mon.Task()(&rootctx)
				spans := collect.CollectSpans(rootctx, startPlanetAndTest)
				done(nil)

				outPath := filepath.Join(outDir, sanitizeFileName(planetConfig.Name))
				var data bytes.Buffer

				switch outType {
				default: // also svg
					if outType != "svg" {
						t.Logf("testplanet-monkit: unknown output type %q defaulting to svg", outType)
					}
					outPath += ".test.svg"
					err := present.SpansToSVG(&data, spans)
					if err != nil {
						t.Error(err)
					}

				case "json":
					outPath += ".test.json"
					err := present.SpansToJSON(&data, spans)
					if err != nil {
						t.Error(err)
					}
				}

				err := os.WriteFile(outPath, data.Bytes(), 0644)
				if err != nil {
					log.Error("failed to write svg", zap.String("path", outPath), zap.Error(err))
				}
			}
		})
	}
}

func parseMonkitFlags(s string) map[string]string {
	r := make(map[string]string)
	for _, tag := range strings.Split(s, ",") {
		tokens := strings.SplitN(tag, "=", 2)
		if len(tokens) <= 1 {
			r["type"] = strings.TrimSpace(tag)
			continue
		}
		key, value := strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1])
		r[key] = value
	}
	return r
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

func provisionUplinks(ctx context.Context, t *testing.T, planet *Planet) {
	for _, planetUplink := range planet.Uplinks {
		for _, satellite := range planet.Satellites {
			apiKey := planetUplink.APIKey[satellite.ID()]
			access, err := uplink.RequestAccessWithPassphrase(ctx, satellite.URL(), apiKey.Serialize(), "")
			if err != nil {
				t.Fatalf("%+v", err)
			}
			planetUplink.Access[satellite.ID()] = access
		}
	}
}
