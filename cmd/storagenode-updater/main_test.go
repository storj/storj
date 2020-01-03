// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"archive/zip"
	"compress/flate"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/version"
	"storj.io/storj/versioncontrol"
)

const (
	oldVersion = "v0.19.0"
	newVersion = "v0.19.5"
)

func TestAutoUpdater(t *testing.T) {
	// TODO cleanup `.exe` extension for different OS

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	oldSemVer, err := version.NewSemVer(oldVersion)
	require.NoError(t, err)

	newSemVer, err := version.NewSemVer(newVersion)
	require.NoError(t, err)

	oldInfo := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "",
		Version:    oldSemVer,
		Release:    false,
	}

	// build real bin with old version, will be used for both storagenode and updater
	oldBin := CompileWithVersion(ctx, "storj.io/storj/cmd/storagenode-updater", oldInfo)
	storagenodePath := ctx.File("fake", "storagenode.exe")
	copyBin(ctx, t, oldBin, storagenodePath)

	updaterPath := ctx.File("fake", "storagenode-updater.exe")
	move(t, oldBin, updaterPath)

	// build real storagenode and updater with new version
	newInfo := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "",
		Version:    newSemVer,
		Release:    false,
	}
	newBin := CompileWithVersion(ctx, "storj.io/storj/cmd/storagenode-updater", newInfo)

	updateBins := map[string]string{
		"storagenode":         newBin,
		"storagenode-updater": newBin,
	}

	// run versioncontrol and update zips http servers
	versionControlPeer, cleanupVersionControl := testVersionControlWithUpdates(ctx, t, updateBins)
	defer cleanupVersionControl()

	logPath := ctx.File("storagenode-updater.log")

	// write identity files to disk for use in rollout calculation
	identConfig := testIdentityFiles(ctx, t)

	// run updater (update)
	args := []string{"run",
		"--config-dir", ctx.Dir(),
		"--server-address", "http://" + versionControlPeer.Addr(),
		"--binary-location", storagenodePath,
		"--check-interval", "0s",
		"--identity.cert-path", identConfig.CertPath,
		"--identity.key-path", identConfig.KeyPath,
		"--log", logPath,
	}

	// NB: updater currently uses `log.SetOutput` so all output after that call
	// only goes to the log file.
	out, err := exec.Command(updaterPath, args...).CombinedOutput()
	logData, logErr := ioutil.ReadFile(logPath)
	if assert.NoError(t, logErr) {
		logStr := string(logData)
		t.Log(logStr)
		if !assert.Contains(t, logStr, "storagenode restarted successfully") {
			t.Log(logStr)
		}
		if !assert.Contains(t, logStr, "storagenode-updater restarted successfully") {
			t.Log(logStr)
		}
	} else {
		t.Log(string(out))
	}
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	oldStoragenode := ctx.File("fake", "storagenode"+".old."+oldVersion+".exe")
	oldStoragenodeInfo, err := os.Stat(oldStoragenode)
	require.NoError(t, err)
	require.NotNil(t, oldStoragenodeInfo)
	require.NotZero(t, oldStoragenodeInfo.Size())

	backupUpdater := ctx.File("fake", "storagenode-updater.old.exe")
	backupUpdaterInfo, err := os.Stat(backupUpdater)
	require.NoError(t, err)
	require.NotNil(t, backupUpdaterInfo)
	require.NotZero(t, backupUpdaterInfo.Size())
}

// CompileWithVersion compiles the specified package with the version variables set
// to the passed version info values and returns the executable name.
func CompileWithVersion(ctx *testcontext.Context, pkg string, info version.Info) string {
	ldFlagsX := map[string]string{
		"storj.io/storj/private/version.buildTimestamp":  strconv.Itoa(int(info.Timestamp.Unix())),
		"storj.io/storj/private/version.buildCommitHash": info.CommitHash,
		"storj.io/storj/private/version.buildVersion":    info.Version.String(),
		"storj.io/storj/private/version.buildRelease":    strconv.FormatBool(info.Release),
	}
	return ctx.CompileWithLDFlagsX(pkg, ldFlagsX)
}

func move(t *testing.T, src, dst string) {
	err := os.Rename(src, dst)
	require.NoError(t, err)
}

func copyBin(ctx *testcontext.Context, t *testing.T, src, dst string) {
	s, err := os.Open(src)
	require.NoError(t, err)
	defer ctx.Check(s.Close)

	d, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE, 0755)
	require.NoError(t, err)
	defer ctx.Check(d.Close)

	_, err = io.Copy(d, s)
	require.NoError(t, err)
}

func testIdentityFiles(ctx *testcontext.Context, t *testing.T) identity.Config {
	t.Helper()

	ident, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	identConfig := identity.Config{
		CertPath: ctx.File("identity", "identity.cert"),
		KeyPath:  ctx.File("identity", "identity.Key"),
	}
	err = identConfig.Save(ident)
	require.NoError(t, err)

	configData := fmt.Sprintf(
		"identity.cert-path: %s\nidentity.key-path: %s",
		identConfig.CertPath,
		identConfig.KeyPath,
	)
	err = ioutil.WriteFile(ctx.File("config.yaml"), []byte(configData), 0644)
	require.NoError(t, err)

	return identConfig
}

func testVersionControlWithUpdates(ctx *testcontext.Context, t *testing.T, updateBins map[string]string) (peer *versioncontrol.Peer, cleanup func()) {
	t.Helper()

	var mux http.ServeMux
	for name, src := range updateBins {
		dst := ctx.File("updates", name+".zip")
		zipBin(ctx, t, dst, src)
		zipData, err := ioutil.ReadFile(dst)
		require.NoError(t, err)

		mux.HandleFunc("/"+name, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write(zipData)
			require.NoError(t, err)
		}))
	}

	ts := httptest.NewServer(&mux)

	var randSeed version.RolloutBytes
	testrand.Read(randSeed[:])
	storagenodeSeed := fmt.Sprintf("%x", randSeed)

	testrand.Read(randSeed[:])
	updaterSeed := fmt.Sprintf("%x", randSeed)

	config := &versioncontrol.Config{
		Address: "127.0.0.1:0",
		// NB: this config field is required for versioncontrol to run.
		Versions: versioncontrol.OldVersionConfig{
			Satellite:   "v0.0.1",
			Storagenode: "v0.0.1",
			Uplink:      "v0.0.1",
			Gateway:     "v0.0.1",
			Identity:    "v0.0.1",
		},
		Binary: versioncontrol.ProcessesConfig{
			Storagenode: versioncontrol.ProcessConfig{
				Suggested: versioncontrol.VersionConfig{
					Version: newVersion,
					URL:     ts.URL + "/storagenode",
				},
				Rollout: versioncontrol.RolloutConfig{
					Seed:   storagenodeSeed,
					Cursor: 100,
				},
			},
			StoragenodeUpdater: versioncontrol.ProcessConfig{
				Suggested: versioncontrol.VersionConfig{
					Version: newVersion,
					URL:     ts.URL + "/storagenode-updater",
				},
				Rollout: versioncontrol.RolloutConfig{
					Seed:   updaterSeed,
					Cursor: 100,
				},
			},
		},
	}
	peer, err := versioncontrol.New(zaptest.NewLogger(t), config)
	require.NoError(t, err)
	ctx.Go(func() error {
		return peer.Run(ctx)
	})
	return peer, func() {
		ts.Close()
		ctx.Check(peer.Close)
	}
}

func zipBin(ctx *testcontext.Context, t *testing.T, dst, src string) {
	t.Helper()

	zipFile, err := os.Create(dst)
	require.NoError(t, err)
	defer ctx.Check(zipFile.Close)

	base := filepath.Base(dst)
	base = base[:len(base)-len(".zip")]

	writer := zip.NewWriter(zipFile)
	defer ctx.Check(writer.Close)

	writer.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.NoCompression)
	})

	contents, err := writer.Create(base)
	require.NoError(t, err)

	data, err := ioutil.ReadFile(src)
	require.NoError(t, err)

	_, err = contents.Write(data)
	require.NoError(t, err)
}
