// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testModularConfig() map[string]string {
	return modularSatelliteConfig(modularSatelliteParams{
		Address:          "127.0.0.1:10000",
		PrivateAddress:   "127.0.0.1:10001",
		ConsoleAddress:   "127.0.0.1:10002",
		AdminAddress:     "127.0.0.1:10005",
		SatelliteDB:      "postgres://localhost/sim?sslmode=disable&options=--search_path=satellite/0",
		MetabaseDB:       "postgres://localhost/sim?sslmode=disable&options=--search_path=satellite/0/meta",
		RedisAddress:     "127.0.0.1:10004",
		RedisStartDB:     0,
		JobqNodeURL:      "12D3KooW@127.0.0.1:10010",
		MailTemplatePath: "/src/storj/web/satellite/static/emails",
		ConsoleStaticDir: "/src/storj/web/satellite/",
		AdminStaticDir:   "/src/storj/satellite/admin/ui/build",
	})
}

func TestModularSatelliteConfig(t *testing.T) {
	config := testModularConfig()

	// addresses and databases
	require.Equal(t, "127.0.0.1:10000", config["server2.address"])
	require.Equal(t, "127.0.0.1:10001", config["server2.private-address"])
	require.Equal(t, "127.0.0.1:10002", config["console.address"])
	require.Equal(t, "127.0.0.1:10005", config["admin.address"])
	require.Equal(t, "postgres://localhost/sim?sslmode=disable&options=--search_path=satellite/0", config["database-options.url"])
	require.Equal(t, "postgres://localhost/sim?sslmode=disable&options=--search_path=satellite/0/meta", config["metainfo.database-url"])
	require.Equal(t, "redis://127.0.0.1:10004?db=0", config["live-accounting.storage-backend"])
	require.Equal(t, "redis://127.0.0.1:10004?db=1", config["server.revocation-dburl"])
	require.Equal(t, "12D3KooW@127.0.0.1:10010", config["jobq.server-node-url"])
	require.Equal(t, "127.0.0.1:0", config["debug.addr"])
	require.Equal(t, "127.0.0.1:0", config["healthcheck.address"])
	require.Equal(t, "true", config["key-management.mock-client"])
	require.Equal(t, "1:secretversion1,12345", config["key-management.key-infos"])

	// the dev values a local network needs (cmd/satellite got them from --defaults dev)
	require.Equal(t, "true", config["contact.allow-private-ip"])
	require.Equal(t, "false", config["overlay.node.distinct-ip"])
	require.Equal(t, "0", config["overlay.minimum-new-node-id-difficulty"])
	require.Equal(t, "4/6/8/10-256 B", config["metainfo.rs"])
	require.Equal(t, "simulate", config["mail.auth-type"])
	require.Equal(t, "false", config["console.signup-activation-code-enabled"])
	require.Equal(t, "true", config["console.open-registration-enabled"])

	// no classic-only keys
	for _, key := range []string{"server.address", "server.private-address", "database", "defaults", "config-dir", "identity-dir"} {
		require.NotContains(t, config, key)
	}
}

func TestWriteFlatYAML(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, writeFlatYAML(filepath.Join(dir, "config.yaml"), map[string]string{
		"server2.address":                   "127.0.0.1:10000",
		"mail.from":                         "Storj <test@storj.io>",
		"console.open-registration-enabled": "true",
		"metainfo.rs":                       "4/6/8/10-256 B",
	}))

	// readConfigString is what storj-sim uses to read values back from config.yaml
	var value string
	require.NoError(t, readConfigString(&value, dir, "server2.address"))
	require.Equal(t, "127.0.0.1:10000", value)
	require.NoError(t, readConfigString(&value, dir, "mail.from"))
	require.Equal(t, "Storj <test@storj.io>", value)
	require.NoError(t, readConfigString(&value, dir, "console.open-registration-enabled"))
	require.Equal(t, "true", value)
	require.NoError(t, readConfigString(&value, dir, "metainfo.rs"))
	require.Equal(t, "4/6/8/10-256 B", value)
}

func TestEnsureModularConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	config := testModularConfig()

	// a config.yaml created by `satellite setup` of a previous release
	classic := "# metainfo.rs: 4/6/8/10-256 B\nserver.address: 127.0.0.1:10000\nconsole.address: 127.0.0.1:10002\ndatabase: postgres://localhost/sim\n"
	require.NoError(t, os.WriteFile(path, []byte(classic), 0644))

	require.NoError(t, ensureModularConfig(dir, config))

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, len(content) > len(classic))
	require.Equal(t, classic, string(content[:len(classic)]), "the classic config must be kept as is")

	var value string
	require.NoError(t, readConfigString(&value, dir, "server2.address"))
	require.Equal(t, "127.0.0.1:10000", value)
	require.NoError(t, readConfigString(&value, dir, "contact.allow-private-ip"))
	require.Equal(t, "true", value)
	// keys present in the classic config are not duplicated (viper would fail on duplicate keys)
	require.Equal(t, 1, countLines(t, content, "console.address:"))

	// idempotent
	require.NoError(t, ensureModularConfig(dir, config))
	again, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(content), string(again))

	// a config written for the modular satellite is left alone
	require.NoError(t, writeFlatYAML(path, config))
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, ensureModularConfig(dir, config))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
}

func countLines(t *testing.T, content []byte, prefix string) int {
	t.Helper()
	count := 0
	for _, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}
