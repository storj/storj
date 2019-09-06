// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/version"
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
	var versionsJson string
	mux.HandleFunc("/versions/release", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, versionsJson)
	}))

	content, err = ioutil.ReadFile("testdata/fake-storagenode.zip")
	require.NoError(t, err)
	mux.HandleFunc("/download", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))

	ts := httptest.NewServer(&mux)
	defer ts.Close()

	suggested, err := version.NewSemVer("v0.19.5")
	require.NoError(t, err)
	response := version.Response{
		Processes: version.Processes{
			Storagenode: version.Process{
				Suggested: version.Version{
					Version: suggested.String(),
					URL:     ts.URL + "/download",
				},
			},
		},
	}
	data, err := json.Marshal(response)
	require.NoError(t, err)
	versionsJson = string(data)

	args := make([]string, 0)
	args = append(args, "run")
	args = append(args, "main.go")
	args = append(args, "run")
	args = append(args, "--version-url")
	args = append(args, ts.URL+"/versions/release")
	args = append(args, "--binary-location")
	args = append(args, tmpExec)
	args = append(args, "--interval")
	args = append(args, "0")

	out, err := exec.Command("go", args...).CombinedOutput()
	require.NoError(t, err)

	if !assert.Contains(t, string(out), "restarted successfully") {
		t.Log(out)
	}
}
