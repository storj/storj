// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/version"
)

func TestRecovery(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.SkipNow()
	}
	ctx := testcontext.New(t)
	// TODO: update zip files are somehow still "...being used by another process"
	//defer ctx.Cleanup()

	// NB: subsequent calls to `compileFakeBin` overwrite the previous output.
	// build bad fake updater with new version
	fakeBadNewBin := compileFakeBin(ctx, newVersion, "1")

	updateBins := map[string]string{
		"storagenode-updater": fakeBadNewBin,
	}

	// run versioncontrol and update zips http servers
	_, cleanupVersionControl := testVersionControlWithUpdates(ctx, t, updateBins)
	defer cleanupVersionControl()

	// build real updater with old version
	ver, err := version.NewSemVer(oldVersion)
	require.NoError(t, err)

	info := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "",
		Version:    ver,
		Release:    false,
	}
	oldRealUpdater := ctx.CompileWithVersion("storj.io/storj/cmd/storagenode-updater", info)

	// modify Product.wxs
	restoreProductWix := modifyProductWix()
	defer ctx.Check(restoreProductWix)

	installerDir, err := filepath.Abs(filepath.Join("..", "..", "installer", "windows"))
	require.NoError(t, err)

	installBat := filepath.Join(installerDir, "install.bat")

	updaterToInstall := filepath.Join(installerDir, "storagenode-updater.exe")
	storagenodeToInstall := filepath.Join(installerDir, "storagenode.exe")

	msiDir := filepath.Join(installerDir, "bin", "Release")
	msiPath := filepath.Join(msiDir, "storagenode.msi")
	installLog := filepath.Join(msiDir, "install.log")

	// copy binaries to be installed to installer directory and build installer
	copyBin(ctx, t, updaterToInstall, oldRealUpdater)

	// build fake updater with new version
	fakeNewBin := compileFakeBin(ctx, newVersion, "0")
	copyBin(ctx, t, storagenodeToInstall, fakeNewBin)
	defer func() {
		err := os.Remove(updaterToInstall)
		assert.NoError(t, err)

		err = os.Remove(storagenodeToInstall)
		assert.NoError(t, err)
	}()

	args := []string{
		filepath.Join(installerDir, "windows.sln"),
		"/t:Build",
		"/p:Configuration=Release",
	}
	msbuildOut, err := exec.Command("msbuild", args...).CombinedOutput()
	if !assert.NoError(t, err) {
		t.Log(string(msbuildOut))
		t.Fatal(err)
	}
	defer func() {
		err := os.Remove(msiPath)
		assert.NoError(t, err)
	}()

	installOut, err := exec.Command(installBat, msiPath).CombinedOutput()
	if !assert.NoError(t, err) {
		installLogData, err := ioutil.ReadFile(installLog)
		if assert.NoError(t, err) {
			t.Logf("install log:\n%s", string(installLogData))
		}
		t.Log(string(installOut))
		t.Fatal(err)
	}

	// TODO: better way to do this?
	//installDir := `C:\Program Files\Storj\Storage Node\`
	//storagenodeLog := installDir + "storagenode.log"
	//updaterLog := installDir + "storagenode-updater.log"

	//// run updater recovery
	//args := []string{"recover"}
	//args = append(args, "--log", storagenodeLog)
	//
	//out, err := exec.Command(oldRealUpdater, args...).CombinedOutput()
	//result := string(out)
	//if !assert.NoError(t, err) {
	//	t.Log(result)
	//	t.Fatal(err)
	//}

	//// NB: updater currently uses `log.SetOutput` so all output after that call
	//// only goes to the log file.
	//logData, logErr := ioutil.ReadFile(storagenodeLog)
	//if assert.NoError(t, logErr) {
	//	logStr := string(logData)
	//	t.Log(logStr)
	//	if !assert.Contains(t, logStr, "storagenode restarted successfully") {
	//		t.Log(logStr)
	//	}
	//	if !assert.Contains(t, logStr, "storagenode-updater restarted successfully") {
	//		t.Log(logStr)
	//	}
	//}
	//if !assert.NoError(t, err) {
	//	t.FailNow()
	//}
}

func modifyProductWix() func() error {
	// scan through file
	// write each line to new file
	// check for `Id="Storagenodeupdater"` and set flag true
	// if flag true when `Arguments=...` encountered, write string replacement
	return func() error {return nil}
}
