// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package windows

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/version"
	"storj.io/storj/storagenode"
)

var (
	compileContext *testcontext.Context
	installerDir   string
	storagenodeBin string
	updaterBin     string
	msiPath        string

	testVersion    = "v0.0.1"
	storagenodePkg = "storj.io/storj/cmd/storagenode"
	updaterPkg     = "storj.io/storj/cmd/storagenode-updater"
)

/* NB: rather than reimplement testcontext#CompileWithVersion, `TestCompile`
builds the binaries and installer once and tests that depend on them
can call `requireInstaller`.
*/
func TestMain(m *testing.M) {
	var err error
	installerDir, err = filepath.Abs(filepath.Join("..", "..", "installer", "windows"))
	if err != nil {
		panic(err)
	}

	storagenodeBin = filepath.Join(installerDir, "storagenode.exe")
	updaterBin = filepath.Join(installerDir, "storagenode-updater.exe")

	msiDir := filepath.Join(installerDir, "bin", "Release")
	msiPath = filepath.Join(msiDir, "storagenode.msi")

	status := m.Run()

	compileContext.Cleanup()

	os.Exit(status)
}

func TestCompileDependencies(t *testing.T) {
	compileContext = testcontext.New(t)

	ver, err := version.NewSemVer(testVersion)
	require.NoError(t, err)

	versionInfo := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "",
		Version:    ver,
		Release:    false,
	}

	tempBin := compileContext.CompileWithVersion(storagenodePkg, versionInfo)
	copyBin(compileContext, t, tempBin, storagenodeBin)

	tempBin = compileContext.CompileWithVersion(updaterPkg, versionInfo)
	copyBin(compileContext, t, tempBin, updaterBin)

	// TODO: goversioninfo stuff so msbuild won't fail

	for name, bin := range map[string]string{
		"storagenode":         storagenodeBin,
		"storagenode-updater": updaterBin,
	} {
		if bin == "" {
			t.Fatalf("empty %s binary path", name)
		}
		if _, err := os.Stat(bin); err != nil {
			t.Fatal(err)
		}
	}

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
}

func TestInstaller_Config(t *testing.T) {
	requireInstaller(t)

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	installLog := ctx.File("install.log")
	installDir := ctx.Dir("install")
	configFile := ctx.File("install", "config.yaml")

	walletAddr := "0x0000000000000000000000000000000000000000"
	email := "user@mail.test"
	publicAddr := "127.0.0.1:0"

	args := []string{
		"/i", msiPath,
		"/passive", "/qb",
		"/norestart",
		"/log", installLog,
		fmt.Sprintf(`INSTALLFOLDER="%s"`, installDir),
		fmt.Sprintf(`STORJ_WALLET="%s"`, walletAddr),
		fmt.Sprintf(`STORJ_EMAIL="%s"`, email),
		fmt.Sprintf(`STORJ_PUBLIC_ADDRESSS="%s"`, publicAddr),
	}
	installOut, err := exec.Command("msiexec", args...).CombinedOutput()
	if !assert.NoError(t, err) {
		installLogData, err := ioutil.ReadFile(installLog)
		if assert.NoError(t, err) {
			t.Logf("MSIExec install.log:\n====================\n%s", string(installLogData))
		}
		t.Logf("MSIExec output:\n===============\n%s", string(installOut))
		t.Fatalf("MSIExec error:\n==============%s", err)
	}

	t.Logf("installDir: %s", installDir)
	files, err := ioutil.ReadDir(installDir)
	require.NoError(t, err)
	for _, f := range files {
		t.Log(f.Name())
	}

	configData, err := ioutil.ReadFile(configFile)
	require.NoError(t, err)

	var config storagenode.Config
	err = yaml.Unmarshal(configData, config)
	require.NoError(t, err)

	t.Logf("config:\n%+v", config)
}

func requireInstaller(t *testing.T) {
	if msiPath == "" {
		t.Fatalf("empty msi path")
	}
	if _, err := os.Stat(msiPath); err != nil {
		t.Fatal(err)
	}
}

// TODO: move to internal / consolidate with cmd/storagnode-updater/main_test's `coypBin`?
// TODO: swap src/dst arg positions
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
