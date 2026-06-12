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
