// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"storj.io/storj/internal/testplanet"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
)

var defaultLibPath string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	defaultLibPath = filepath.Join(filepath.Dir(thisFile), "uplink-cgo.so")
}

func TestSanity(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// WIP -- set up testplanet...
	//testplanet.New


	assert.True(t, false)
	testBinPath := ctx.CompileC(defaultLibPath, filepath.Join(filepath.Dir(defaultLibPath), "tests", "*.c"))

	cmd := exec.Command(testBinPath)
	out, err := cmd.CombinedOutput()
	_, _ = out, err
	fmt.Println(out)
	fmt.Println(err)
}
