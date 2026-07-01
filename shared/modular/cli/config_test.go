// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSubtree(t *testing.T) {
	currentDir, err := os.Getwd()
	require.NoError(t, err)
	cfg := ConfigSupport{
		configDir: filepath.Join(currentDir, "testdata"),
	}

	var sc []ServerConfig
	err = cfg.GetSubtree("prometheus.servers", &sc)
	require.NoError(t, err)
	require.Equal(t, "victoria", sc[0].Name)
}

type ServerConfig struct {
	Name       string
	URL        string
	CaCertPath string
}

func TestGetSubtreeMergesSecrets(t *testing.T) {
	dir := t.TempDir()
	config := "" +
		"db:\n" +
		"  host: config-host\n" +
		"  port: 5432\n"
	secrets := "" +
		"db:\n" +
		"  host: secret-host\n" +
		"  password: s3cr3t\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(secrets), 0644))

	cfg := &ConfigSupport{configDir: dir}

	type DBConfig struct {
		Host     string
		Port     int
		Password string
	}
	var db DBConfig
	require.NoError(t, cfg.GetSubtree("db", &db))

	// config.yaml wins on conflicting keys, secrets.yaml fills in the rest.
	require.Equal(t, "config-host", db.Host)
	require.Equal(t, 5432, db.Port)
	require.Equal(t, "s3cr3t", db.Password)
}

func TestGetValue(t *testing.T) {
	dir := t.TempDir()
	yaml := "" +
		"single-string: hello\n" +
		"string-list:\n" +
		"  - first@example.com\n" +
		"  - second@example.com\n" +
		"one-element-list:\n" +
		"  - only@example.com\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0644))

	cfg := &ConfigSupport{configDir: dir}

	v, err := cfg.GetValue("single-string")
	require.NoError(t, err)
	require.Equal(t, []string{"hello"}, v)

	// list values must be comma-joined (the form the binder splits back into a
	// []string), not rendered with fmt's "[a b]" slice representation.
	v, err = cfg.GetValue("string-list")
	require.NoError(t, err)
	require.Equal(t, []string{"first@example.com,second@example.com"}, v)

	v, err = cfg.GetValue("one-element-list")
	require.NoError(t, err)
	require.Equal(t, []string{"only@example.com"}, v)
}

func TestGetValueMergesSecrets(t *testing.T) {
	dir := t.TempDir()
	config := "" +
		"shared-key: from-config\n" +
		"config-only: c-value\n"
	secrets := "" +
		"shared-key: from-secrets\n" +
		"secret-only: s-value\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(config), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.yaml"), []byte(secrets), 0644))

	cfg := &ConfigSupport{configDir: dir}

	// config.yaml takes precedence on conflicting keys.
	v, err := cfg.GetValue("shared-key")
	require.NoError(t, err)
	require.Equal(t, []string{"from-config"}, v)

	v, err = cfg.GetValue("config-only")
	require.NoError(t, err)
	require.Equal(t, []string{"c-value"}, v)

	v, err = cfg.GetValue("secret-only")
	require.NoError(t, err)
	require.Equal(t, []string{"s-value"}, v)
}
