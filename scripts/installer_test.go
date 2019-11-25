// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"storj.io/storj/private/testcontext"
)

var (
	// TODO: make this more dynamic and/or use versioncontrol server?
	// (NB: can't use versioncontrol server until updater process is added to response)
	downloadVersion    = "v0.26.2"
	buildInstallerOnce = sync.Once{}
	msiBaseArgs        = []string{
		"/quiet", "/qn",
		"/norestart",
	}

	msiPath = flag.String("msi", "", "path to the msi to use")
)

func TestInstaller_Config(t *testing.T) {
	if *msiPath == "" {
		t.Fatal("no msi passed, use `go test ... -args -msi <msi path>`")
	}

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	tryUninstall(t, ctx)

	installDir := ctx.Dir("install")
	configPath := ctx.File("install", "config.yaml")

	walletAddr := "0x0000000000000000000000000000000000000000"
	email := "user@mail.test"
	publicAddr := "127.0.0.1:10000"

	args := []string{
		fmt.Sprintf("INSTALLFOLDER=%s", installDir),
		//fmt.Sprintf("STORJ_IDENTITYDIR=%s", installDir),
		fmt.Sprintf("STORJ_WALLET=%s", walletAddr),
		fmt.Sprintf("STORJ_EMAIL=%s", email),
		fmt.Sprintf("STORJ_PUBLIC_ADDRESS=%s", publicAddr),
	}
	install(t, ctx, args...)
	defer requireUninstall(t, ctx)

	configFile, err := os.Open(configPath)
	require.NoError(t, err)
	defer configFile.Close()

	// NB: strip empty lines and comments
	configBuf := bytes.Buffer{}
	scanner := bufio.NewScanner(configFile)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.Trim(line, " \t\n")
		out := append(scanner.Bytes(), byte('\n'))
		if len(line) == 0 {
			continue
		}
		if !strings.HasPrefix(line, "#") {
			_, err := configBuf.Write(out)
			require.NoError(t, err)
		}
	}
	if err := scanner.Err(); err != nil {
		require.NoError(t, err)
	}

	// TODO: require identity file paths
	//certPath := ctx.File("install", "identity.cert")
	//keyPath := ctx.File("install", "identity.key")

	//expectedCertPath := fmt.Sprintf("identity.cert-path: %s", certPath)
	//expectedKeyPath := fmt.Sprintf("identity.key-path: %s", keyPath)
	expectedEmail := fmt.Sprintf("operator.email: %s", email)
	expectedWallet := fmt.Sprintf("operator.wallet: \"%s\"", walletAddr)
	expectedAddr := fmt.Sprintf("contact.external-address: %s", publicAddr)

	configStr := configBuf.String()
	//require.Contains(t, configStr, expectedCertPath)
	//require.Contains(t, configStr, expectedKeyPath)
	require.Contains(t, configStr, expectedEmail)
	require.Contains(t, configStr, expectedWallet)
	require.Contains(t, configStr, expectedAddr)
}

func install(t *testing.T, ctx *testcontext.Context, args ...string) {
	logPath := ctx.File("install.log")
	args = append(append([]string{
		"/i", *msiPath,
		"/log", logPath,
	}, msiBaseArgs...), args...)

	installOut, err := exec.Command("msiexec", args...).CombinedOutput()
	if !assert.NoError(t, err) {
		installLogData, err := ioutil.ReadFile(logPath)
		if assert.NoError(t, err) {
			t.Logf("MSIExec log:\n============================\n%s", string(installLogData))
		}
		t.Logf("MSIExec output:\n============================\n%s", string(installOut))
		t.Fatal()
	}
}

func tryUninstall(t *testing.T, ctx *testcontext.Context) {
	_, err := uninstall(t, ctx).CombinedOutput()
	if err != nil {
		t.Logf("WARN: tried but failed to uninstall from: %s", *msiPath)
	}
}

func requireUninstall(t *testing.T, ctx *testcontext.Context) {
	logPath := ctx.File("uninstall.log")
	uninstallOut, err := uninstall(t, ctx).CombinedOutput()
	if err != nil {
		uninstallLogData, err := ioutil.ReadFile(logPath)
		if assert.NoError(t, err) {
			t.Logf("MSIExec log:\n============================\n%s", string(uninstallLogData))
		}
		t.Logf("MSIExec output:\n============================\n%s", string(uninstallOut))
	}
}

func uninstall(t *testing.T, ctx *testcontext.Context) *exec.Cmd {
	stopServices(t, ctx, "storagenode", "storagenode-updater")

	args := append([]string{"/uninstall", *msiPath}, msiBaseArgs...)
	return exec.Command("msiexec", args...)
}

func stopServices(t *testing.T, ctx *testcontext.Context, names ...string) {
	t.Helper()

	serviceMgr, err := mgr.Connect()
	require.NoError(t, err)

	group := new(errgroup.Group)
	for _, name := range names {
		service, err := serviceMgr.OpenService(name)
		require.NoError(t, err)
		defer ctx.Check(service.Close)

		_, err = service.Control(svc.Stop)
		require.NoError(t, err)

		group.Go(waitForStop(service))
	}

	err = group.Wait()
	require.NoError(t, err)
}

func waitForStop(service *mgr.Service) (func() error) {
	return func() error {
		ctx, _ := context.WithTimeout(context.Background(), 30*time.Second)
		for {
			status, err := service.Query()
			if err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			switch status.State {
			case svc.Stopped:
				return nil
			default:
				time.Sleep(500 * time.Millisecond)
			}
		}
	}
}
