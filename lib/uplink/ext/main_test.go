// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite/console"
)

var cLibDir, cSrcDir, cHeadersDir, libuplink string

func init() {
	_, thisFile, _, _ := runtime.Caller(0)
	cLibDir = filepath.Join(filepath.Dir(thisFile), "c")
	cSrcDir = filepath.Join(cLibDir, "src")
	cHeadersDir = filepath.Join(cLibDir, "headers")
	libuplink = filepath.Join(cLibDir, "..", "uplink-cgo.so")
}

// TODO: split c test up into multiple suites, each of which gets a go test function.
func TestAllCTests(t *testing.T) {
	ctx := testcontext.New(t)
	// defer ctx.Cleanup()

	planet, err := testplanet.NewWithLogger(zap.NewNop(), 1, 8, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// TODO: support multiple satelllites?
	projectName := t.Name()
	APIKey := console.APIKeyFromBytes([]byte(projectName))
	consoleDB := planet.Satellites[0].DB.Console()

	project, err := consoleDB.Projects().Insert(
		context.Background(),
		&console.Project{
			Name: projectName,
		},
	)
	require.NoError(t, err)
	require.NotNil(t, project)

	_, err = consoleDB.APIKeys().Create(
		context.Background(),
		*APIKey,
		console.APIKeyInfo{
			Name:      "root",
			ProjectID: project.ID,
		},
	)
	require.NoError(t, err)

	fmt.Printf("libuplink: %s\n", libuplink)
	testBinPath := ctx.CompileC(
		libuplink,
		filepath.Join(cSrcDir, "*.c"),
		filepath.Join(cLibDir, "pb", "*.c"),
		filepath.Join(cLibDir, "tests", "*.c"),
	)

	cmd := exec.Command(testBinPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("SATELLITEADDR=%s", planet.Satellites[0].Addr()),
		fmt.Sprintf("APIKEY=%s", APIKey.String()),
	)

	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	t.Log(string(out))
}
