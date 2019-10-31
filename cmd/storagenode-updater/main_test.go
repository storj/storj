// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/versioncontrol"
)

func TestAutoUpdater(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testFiles := []struct {
		src   string
		dst   string
		perms os.FileMode
	}{
		{
			"testdata/fake-storagenode",
			ctx.File("storagenode"),
			0755,
		},
		{
			"testdata/fake-ident.cert",
			ctx.File("identity.cert"),
			0644,
		},
		{
			"testdata/fake-ident.key",
			ctx.File("identity.key"),
			0600,
		},
	}

	for _, file := range testFiles {
		content, err := ioutil.ReadFile(file.src)
		require.NoError(t, err)

		err = ioutil.WriteFile(file.dst, content, file.perms)
		require.NoError(t, err)
	}

	var mux http.ServeMux
	content, err := ioutil.ReadFile("testdata/fake-storagenode.zip")
	require.NoError(t, err)
	mux.HandleFunc("/download", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(content)
		require.NoError(t, err)
	}))

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
					URL:     ts.URL + "/download",
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

	args := make([]string, 0)
	args = append(args, "run")
	args = append(args, "main.go")
	args = append(args, "run")
	args = append(args, "--server-address")
	args = append(args, "http://"+peer.Addr())
	args = append(args, "--binary-location")
	args = append(args, testFiles[0].dst)
	args = append(args, "--check-interval")
	args = append(args, "0s")
	args = append(args, "--identity.cert-path")
	args = append(args, testFiles[1].dst)
	args = append(args, "--identity.key-path")
	args = append(args, testFiles[2].dst)

	out, err := exec.Command("go", args...).CombinedOutput()
	result := string(out)
	if !assert.NoError(t, err) {
		t.Log(result)
		t.Fatal(err)
	}

	if !assert.Contains(t, result, "restarted successfully") {
		t.Log(result)
	}
}
