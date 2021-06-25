// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/processgroup"
	"storj.io/common/testcontext"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/private/testplanet"
)

type EdgeInfo struct {
	AccessKey   string
	SecretKey   string
	GatewayAddr string
	AuthSvcAddr string
}

var counter int64

// Test defines common services for uitests.
type EdgeTest func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser, edgeinfo EdgeInfo)

// Run starts a new UI test.
func RunWithEdge(t *testing.T, test EdgeTest) {
	Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]

		// TODO: make address not hardcoded the address selection here
		// may conflict with some automatically bound address.
		gatewayAddr := fmt.Sprintf("127.0.0.1:1100%d", atomic.AddInt64(&counter, 1))
		authSvcAddr := fmt.Sprintf("127.0.0.1:1100%d", atomic.AddInt64(&counter, 1))
		satelliteURL := planet.Satellites[0].URL()

		gatewayExe := compileAt(t, ctx, "../../cmd", "storj.io/gateway-mt/cmd/gateway-mt")
		authSvcExe := compileAt(t, ctx, "../../cmd", "storj.io/gateway-mt/cmd/authservice")

		authSvc, err := startAuthSvc(t, authSvcExe, authSvcAddr, gatewayAddr, satelliteURL)
		require.NoError(t, err)
		defer func() { processgroup.Kill(authSvc) }()

		accessKey, secretKey, _, err := cmd.RegisterAccess(ctx, access, "http://"+authSvcAddr, false, 15*time.Second)
		require.NoError(t, err)

		gateway, err := startGateway(t, gatewayExe, gatewayAddr, authSvcAddr)
		require.NoError(t, err)
		defer func() { processgroup.Kill(gateway) }()

		edgeInfo := EdgeInfo{AccessKey: accessKey, SecretKey: secretKey, GatewayAddr: gatewayAddr, AuthSvcAddr: authSvcAddr}
		test(t, ctx, planet, browser, edgeInfo)
	})
}

func compileAt(t *testing.T, ctx *testcontext.Context, workDir string, pkg string) string {
	t.Helper()

	var binName string
	if pkg == "" {
		dir, _ := os.Getwd()
		binName = path.Base(dir)
	} else {
		binName = path.Base(pkg)
	}

	if absDir, err := filepath.Abs(workDir); err == nil {
		workDir = absDir
	} else {
		t.Log(err)
	}

	exe := ctx.File("build", binName+".exe")

	/* #nosec G204 */ // This package is only used for test
	cmd := exec.Command("go", "build", "-race", "-tags=unittest", "-o", exe, pkg)
	t.Log("exec:", cmd.Args, "dir:", workDir)
	cmd.Dir = workDir

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(out))
		t.Fatal(err)
	}

	return exe
}

func startAuthSvc(t *testing.T, exe, authSvcAddress, gatewayAddress, satelliteURL string, moreFlags ...string) (*exec.Cmd, error) {
	args := append([]string{"run",
		"--auth-token", "super-secret",
		"--allowed-satellites", satelliteURL,
		"--endpoint", "http://" + gatewayAddress,
		"--listen-addr", authSvcAddress,
	}, moreFlags...)

	authSvc := exec.Command(exe, args...)

	log := zaptest.NewLogger(t)
	authSvc.Stdout = logWriter{log.Named("authsvc:stdout")}
	authSvc.Stderr = logWriter{log.Named("authsvc:stderr")}

	err := authSvc.Start()
	if err != nil {
		return nil, err
	}

	err = waitForAuthSvcStart(authSvcAddress, 5*time.Second, authSvc)
	if err != nil {
		killErr := authSvc.Process.Kill()
		return nil, errs.Combine(err, killErr)
	}

	return authSvc, nil
}

// waitForAuthSvcStart will monitor starting when we are able to start the process.
func waitForAuthSvcStart(authSvcAddress string, maxStartupWait time.Duration, cmd *exec.Cmd) error {
	start := time.Now()
	for {
		_, err := http.Get("http://" + authSvcAddress)
		if err == nil {
			return nil
		}

		// wait a bit before retrying to reduce load
		time.Sleep(50 * time.Millisecond)

		if time.Since(start) > maxStartupWait {
			return cmdErr("AuthSvc", "start", authSvcAddress, maxStartupWait, cmd)
		}
	}
}

func startGateway(t *testing.T, exe, gatewayAddress,
	authSvcAddress string, moreFlags ...string) (*exec.Cmd, error) {
	args := append([]string{"run",
		"--server.address", gatewayAddress,
		"--auth-token", "super-secret",
		"--auth-url", "http://" + authSvcAddress,
		"--domain-name", "localhost",
	}, moreFlags...)

	gateway := exec.Command(exe, args...)

	log := zaptest.NewLogger(t)
	gateway.Stdout = logWriter{log.Named("gateway:stdout")}
	gateway.Stderr = logWriter{log.Named("gateway:stderr")}

	err := gateway.Start()
	if err != nil {
		return nil, err
	}

	err = waitForGatewayStart(gatewayAddress, 5*time.Second, gateway)
	if err != nil {
		killErr := gateway.Process.Kill()
		return nil, errs.Combine(err, killErr)
	}

	return gateway, nil
}

func cmdErr(app, action, address string, wait time.Duration, cmd *exec.Cmd) error {
	return fmt.Errorf("%s [%s] did not %s in required time %v : %s",
		app, address, action, wait, strings.Join(cmd.Args, " "))
}

// waitForGatewayStart will monitor starting when we are able to start the process.
func waitForGatewayStart(gatewayAddress string, maxStartupWait time.Duration, cmd *exec.Cmd) error {
	start := time.Now()
	for {
		_, err := http.Get("http://" + gatewayAddress + "/-/health")
		if err == nil {
			return nil
		}

		// wait a bit before retrying to reduce load
		time.Sleep(50 * time.Millisecond)

		if time.Since(start) > maxStartupWait {
			return cmdErr("Gateway", "start", gatewayAddress, maxStartupWait, cmd)
		}
	}
}

type logWriter struct{ log *zap.Logger }

func (log logWriter) Write(p []byte) (n int, err error) {
	log.log.Debug(string(p))
	return len(p), nil
}
