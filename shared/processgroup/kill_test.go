// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package processgroup_test

import (
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/processgroup"
)

func TestProcessGroup(t *testing.T) {
	ctx := testcontext.New(t)

	source := ctx.File("main.go")
	binary := ctx.File("main.exe")
	err := os.WriteFile(source, []byte(code), 0644)
	require.NoError(t, err)

	{
		/* #nosec G204 */ // This is a test and both parameters' values are controlled
		cmd := exec.Command("go", "build", "-o", binary, source)
		cmd.Dir = ctx.Dir()

		_, err := cmd.CombinedOutput()
		require.NoError(t, err)
	}

	{

		/* #nosec G204 */ // This is a test and the parameter's values is controlled
		cmd := exec.Command(binary)
		cmd.Dir = ctx.Dir()
		cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
		processgroup.Setup(cmd)

		started := time.Now()
		err := cmd.Start()
		require.NoError(t, err)
		processgroup.Kill(cmd)

		_ = cmd.Wait() // since we kill it, we might get an error
		duration := time.Since(started)

		require.Truef(t, duration < 10*time.Second, "completed in %s", duration)
	}
}

const code = `package main

import "time"

func main() {
	time.Sleep(20*time.Second)
}
`
