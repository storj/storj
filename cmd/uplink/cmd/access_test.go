// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package cmd_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/uplink"
)

const testAccess = "12edqrJX1V243n5fWtUrwpMQXL8gKdY2wbyqRPSG3rsA1tzmZiQjtCyF896egifN2C2qdY6g5S1t6e8iDhMUon9Pb7HdecBFheAcvmN8652mqu8hRx5zcTUaRTWfFCKS2S6DHmTeqPUHJLEp6cJGXNHcdqegcKfeahVZGP4rTagHvFGEraXjYRJ3knAcWDGW6BxACqogEWez6r274JiUBfs4yRSbRNRqUEURd28CwDXMSHLRKKA7TEDKEdQ"

func TestRegisterAccess(t *testing.T) {
	ctx := testcontext.New(t)

	// mock the auth service
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, `{"access_key_id":"1", "secret_key":"2", "endpoint":"3"}`)
		}))
	defer ts.Close()
	// make sure we get back things
	access, err := uplink.ParseAccess(testAccess)
	require.NoError(t, err)
	accessKey, secretKey, endpoint, err := cmd.RegisterAccess(ctx, access, ts.URL, true, 15*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "1", accessKey)
	assert.Equal(t, "2", secretKey)
	assert.Equal(t, "3", endpoint)
}

func TestRegisterAccessTimeout(t *testing.T) {
	ctx := testcontext.New(t)

	// mock the auth service
	ch := make(chan struct{})
	ts := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-ch
		}))
	defer ts.Close()
	// make sure we get back things
	access, err := uplink.ParseAccess(testAccess)
	require.NoError(t, err)
	accessKey, secretKey, endpoint, err := cmd.RegisterAccess(ctx, access, ts.URL, true, 10*time.Millisecond)
	require.Error(t, err)
	assert.Equal(t, "", accessKey)
	assert.Equal(t, "", secretKey)
	assert.Equal(t, "", endpoint)
	close(ch)
}

func TestAccessImport(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const testAccess = "12edqwjdy4fmoHasYrxLzmu8Ubv8Hsateq1LPYne6Jzd64qCsYgET53eJzhB4L2pWDKBpqMowxt8vqLCbYxu8Qz7BJVH1CvvptRt9omm24k5GAq1R99mgGjtmc6yFLqdEFgdevuQwH5yzXCEEtbuBYYgES8Stb1TnuSiU3sa62bd2G88RRgbTCtwYrB8HZ7CLjYWiWUphw7RNa3NfD1TW6aUJ6E5D1F9AM6sP58X3D4H7tokohs2rqCkwRT"

	uplinkExe := ctx.Compile("storj.io/storj/cmd/uplink")

	output, err := exec.Command(uplinkExe, "--config-dir", ctx.Dir("uplink"), "import", testAccess).CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
}
