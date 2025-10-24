// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"bufio"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"storj.io/common/fpath"
	"storj.io/common/testcontext"
)

//go:generate go test -run TestConfigLock -generate-config-lock
var createLock = flag.Bool("generate-config-lock", false, "")

// TestConfigLock tests or updates the satellite config lock file.
func TestConfigLock(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Minute)
	defer ctx.Cleanup()

	// run the satellite executable to create a config file
	tempDir := ctx.Dir("", "satellite-cfg-lock-")
	satelliteExe := ctx.Compile("storj.io/storj/cmd/satellite")
	satelliteCmd := exec.Command(satelliteExe, "--config-dir", tempDir, "--defaults", "release", "setup")
	out, err := satelliteCmd.CombinedOutput()
	assert.NoErrorf(t, err, "Error running satellite", string(out))
	cleanedupConfig := filepath.Join(tempDir, "config-normalized.yaml")

	// normalize certain OS-specific paths that occurn in the config
	normalizeConfig(t, filepath.Join(tempDir, "config.yaml"), cleanedupConfig, tempDir)

	// either compare or save the lock file
	lockPath := "satellite-config.yaml.lock"
	if *createLock { // update satellite-config.yaml.lock
		// copy using ReadFile/WriteFile, since os.Rename() won't work across drives
		input, err := os.ReadFile(cleanedupConfig)
		assert.NoErrorf(t, err, "Error reading file for move")
		err = os.WriteFile(lockPath, input, 0644)
		assert.NoErrorf(t, err, "Error writing file for move")
	} else { // compare to satellite-config.yaml.lock
		configs1 := readLines(t, lockPath)
		configs2 := readLines(t, cleanedupConfig)
		if diff := cmp.Diff(configs1, configs2); diff != "" {
			t.Errorf(`The satellite config does not match the lock file. (-want +got):\n%s
			NOTIFY the #config-changes channel so they can plan for changing it in the release process.
			Once you have notified them and got their confirmation, you can update the lock file by running:
			"go generate ./satellite"`, diff)
		}
	}
}

// readLines takes a file path and returns the contents split by lines.
func readLines(t *testing.T, filePath string) []string {
	file, err := os.ReadFile(filePath)
	assert.NoErrorf(t, err, "Error opening %s", filePath)
	return strings.Split(strings.ReplaceAll(string(file), "\r\n", "\n"), "\n")
}

// normalizeConfig replace platform specific storj paths as if they ran as Linux root.
func normalizeConfig(t *testing.T, configIn, configOut, tempDir string) {
	in, err := os.Open(configIn)
	assert.NoErrorf(t, err, "Error opening %s", in)
	defer func() { assert.NoErrorf(t, in.Close(), "Error closing %s", in) }()

	out, err := os.Create(configOut)
	assert.NoErrorf(t, err, "Error opening %s", out)
	defer func() { assert.NoErrorf(t, out.Close(), "Error closing %s", out) }()

	scanner := bufio.NewScanner(in)
	writer := bufio.NewWriter(out)
	defer func() { assert.NoErrorf(t, writer.Flush(), "Error flushing %s", out) }()

	appDir := fpath.ApplicationDir()
	for scanner.Scan() {
		line := scanner.Text()
		// fix metrics.app and tracing.app
		line = strings.Replace(line, ".exe", "", 1)
		// fix server.revocation-dburl
		line = strings.Replace(line, tempDir, "testdata", 1)
		// fix identity.cert-path and identity.key-path
		if strings.Contains(line, appDir) {
			line = strings.Replace(line, appDir, "/root/.local/share", 1)
			line = strings.ToLower(strings.ReplaceAll(line, "\\", "/"))
		}
		_, err = writer.WriteString(line + "\n")
		assert.NoErrorf(t, err, "Error writing %s", out)
	}
	assert.NoErrorf(t, scanner.Err(), "Error reading %s", in)
}
