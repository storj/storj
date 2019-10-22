// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/versioncontrol"
)

func TestAutoUpdater(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ident, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	identConfig := identity.Config{
		CertPath: ctx.File("identity.cert"),
		KeyPath:  ctx.File("identity.Key"),
	}
	err = identConfig.Save(ident)
	require.NoError(t, err)

	testFiles := map[string]struct {
		oldBin string
		newZip string
	}{
		"storagenode": {
			"testdata/fake-storagenode",
			"testdata/fake-storagenode.zip",
		},
		"storagenode-updater": {
			"testdata/fake-storagenode-updater",
			"testdata/fake-storagenode-updater.zip",
		},
	}

	var mux http.ServeMux
	for name, file := range testFiles {
		oldBinData, err := ioutil.ReadFile(file.oldBin)
		require.NoError(t, err)

		err = ioutil.WriteFile(ctx.File(name), oldBinData, 0755)
		require.NoError(t, err)

		newZipData, err := ioutil.ReadFile(file.newZip)
		require.NoError(t, err)
		mux.HandleFunc("/" + name, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := w.Write(newZipData)
			require.NoError(t, err)
		}))
	}

	ts := httptest.NewServer(&mux)
	defer ts.Close()

	config := &versioncontrol.Config{
		Address: "127.0.0.1:0",
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
					Version: "0.19.5",
					URL:     ts.URL + "/storagenode",
				},
				Rollout: versioncontrol.RolloutConfig{
					Seed:   "0000000000000000000000000000000000000000000000000000000000000001",
					Cursor: 100,
				},
			},
			StoragenodeUpdater: versioncontrol.ProcessConfig{
				Suggested: versioncontrol.VersionConfig{
					Version: "0.19.5",
					URL:     ts.URL + "/storagenode-updater",
				},
				Rollout: versioncontrol.RolloutConfig{
					Seed:   "0000000000000000000000000000000000000000000000000000000000000001",
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
	defer ctx.Check(peer.Close)

	// NB: same as old fake-storagenode version
	ver, err := version.NewSemVer("v0.19.0")
	require.NoError(t, err)

	info := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "",
		Version:    ver,
		Release:    false,
	}
	updaterBin := ctx.CompileWithVersion("", info)

	args := make([]string, 0)
	args = append(args, "run")
	args = append(args, "--server-address")
	args = append(args, "http://"+peer.Addr())
	args = append(args, "--binary-location")
	args = append(args, ctx.File("storagenode"))
	args = append(args, "--check-interval")
	args = append(args, "0s")
	args = append(args, "--identity.cert-path")
	args = append(args, identConfig.CertPath)
	args = append(args, "--identity.key-path")
	args = append(args, identConfig.KeyPath)

	out, err := exec.Command(updaterBin, args...).CombinedOutput()
	result := string(out)
	if !assert.NoError(t, err) {
		t.Log(result)
		t.Fatal(err)
	}

	if !assert.Contains(t, result, "storagenode restarted successfully") {
		t.Log(result)
	}

	if !assert.Contains(t, result, "storagenode-updater restarted successfully") {
		t.Log(result)
	}
}
