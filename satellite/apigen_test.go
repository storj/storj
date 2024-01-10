// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"storj.io/common/testcontext"
	"storj.io/storj/private/apigen"
)

// TestGeneratedAPIs checks whether the generated APIs are up-to-date.
func TestGeneratedAPIs(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Minute)
	defer ctx.Cleanup()

	type apiCheck struct {
		genPath  string
		genFiles []string
	}

	tempDir := ctx.Dir("satellite-generated-apis")
	rootDir := findModuleRootDir()

	for _, tt := range []apiCheck{
		{
			genPath: "satellite/console/consoleweb/consoleapi/gen",
			genFiles: []string{
				"satellite/console/consoleweb/consoleapi/api.gen.go",
				"satellite/console/consoleweb/consoleapi/apidocs.gen.md",
				"web/satellite/src/api/v0.gen.ts",
			},
		},
		{
			genPath: "satellite/admin/back-office/gen",
			genFiles: []string{
				"satellite/admin/back-office/handlers.gen.go",
				"satellite/admin/back-office/api-docs.gen.md",
				"satellite/admin/back-office/ui/src/api/client.gen.ts",
			},
		},
		{
			genPath: "private/apigen/example",
			genFiles: []string{
				"private/apigen/example/api.gen.go",
				"private/apigen/example/client-api.gen.ts",
				"private/apigen/example/client-api-mock.gen.ts",
				"private/apigen/example/apidocs.gen.md",
			},
		},
	} {
		// 1. generate files to test directory
		cmd := exec.Command("go", "generate", path.Join(rootDir, tt.genPath))
		rootOverrideEnv := fmt.Sprintf("%s=%s", apigen.OutputRootDirEnvOverride, tempDir)
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, rootOverrideEnv)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatal(err, string(out))
		}

		// 2. compare original vs. newly-generated files
		for _, file := range tt.genFiles {
			oldFile := path.Join(rootDir, file)
			newFile := path.Join(tempDir, file)

			originalData := readLines(t, oldFile)
			newData := readLines(t, newFile)
			if diff := cmp.Diff(originalData, newData); diff != "" {
				t.Errorf(`The satellite generated APIs have changed since being re-generated:
%s
Filepath: %s
Regenerate by running one or all of the following:
"go generate ./satellite/console/..."
"go generate ./satellite/admin/back-office/..."
"go generate ./private/apigen/example/..."`, diff, newFile)
			}
		}
	}
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
