// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
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

	content, err := ioutil.ReadFile("testdata/fake-storagenode")
	require.NoError(t, err)

	tmpExec := ctx.File("storagenode")
	err = ioutil.WriteFile(tmpExec, content, 0755)
	require.NoError(t, err)

	var mux http.ServeMux
	content, err = ioutil.ReadFile("testdata/fake-storagenode.zip")
	require.NoError(t, err)
	mux.HandleFunc("/download", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write(content)
		require.NoError(t, err)
	}))

	ts := httptest.NewServer(&mux)
	defer ts.Close()

	config := &versioncontrol.Config{
		Address: "127.0.0.1:0",
		Versions: versioncontrol.ServiceVersions{
			Bootstrap:   "v0.0.1",
			Satellite:   "v0.0.1",
			Storagenode: "v0.0.1",
			Uplink:      "v0.0.1",
			Gateway:     "v0.0.1",
			Identity:    "v0.0.1",
		},
		Binary: versioncontrol.Versions{
			Storagenode: versioncontrol.Binary{
				Suggested: versioncontrol.Version{
					Version: "0.19.5",
					URL:     ts.URL + "/download",
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
	args = append(args, "--version-url")
	args = append(args, "http://"+peer.Addr())
	args = append(args, "--binary-location")
	args = append(args, tmpExec)
	args = append(args, "--interval")
	args = append(args, "0")

	out, err := exec.Command("go", args...).CombinedOutput()
	assert.NoError(t, err)

	result := string(out)
	if !assert.Contains(t, result, "restarted successfully") {
		t.Log(result)
	}
}
