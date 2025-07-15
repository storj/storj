// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/apigen"
)

// TestGeneratedAPIs checks whether the generated APIs are up-to-date.
func TestGeneratedAPIs(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Minute)
	defer ctx.Cleanup()

	tempDir := ctx.Dir()
	rootDir := findModuleRootDir()
	helpText := "Regenerate by running the following:\n"

	for _, genPath := range []string{
		"satellite/console/consoleweb/consoleapi/gen",
		"satellite/console/consoleweb/consoleapi/privateapi/gen",
		"satellite/admin/back-office/gen",
		"private/apigen/example",
	} {
		// generate files to test directory
		helpText += fmt.Sprintf("go generate ./%s\n", genPath)
		cmd := exec.Command("go", "generate", path.Join(rootDir, genPath))
		rootOverrideEnv := fmt.Sprintf("%s=%s", apigen.OutputRootDirEnvOverride, tempDir)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, rootOverrideEnv)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}

	}

	// iterate over all newly-generated files, and compare to originals
	err := filepath.Walk(tempDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		newFile := path
		relativePath := strings.TrimPrefix(path, tempDir)
		oldFile := filepath.Join(rootDir, relativePath)

		originalData := readLines(t, oldFile)
		newData := readLines(t, newFile)
		if diff := cmp.Diff(originalData, newData); diff != "" {
			t.Errorf("The satellite generated APIs have changed since being re-generated:\nPath:\n%s\nDiff:\n%s\n%s", relativePath, diff, helpText)
			return nil
		}
		return nil
	})
	require.NoError(t, err)
}

func findModuleRootDir() string {
	dir, err := os.Getwd()
	if err != nil {
		panic("unable to find current working directory")
	}
	start := dir

	for i := 0; i < 100; i++ {
		if fileExists(filepath.Join(dir, "go.mod")) {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			break
		}
		dir = next
	}

	panic("unable to find go.mod starting from " + start)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
