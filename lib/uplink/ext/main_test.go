// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

var defaultLibPath string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	defaultLibPath = filepath.Join(filepath.Dir(thisFile), "uplink-cgo.so")
}

func TestSanity(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.NewWithLogger(zap.NewNop(), 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	testBinPath := ctx.CompileC(defaultLibPath, filepath.Join(filepath.Dir(defaultLibPath), "tests", "*.c"))

	cmd := exec.Command(testBinPath)
	cmd.Env = append(os.Environ(),
		"SATELLITE=127.0.0.1",
	)

	out, err := cmd.CombinedOutput() 
	require.NoError(t, err)
	require.NotContains(t, string(out), "FAIL")
}
