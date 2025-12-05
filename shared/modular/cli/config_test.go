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
