// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	console "storj.io/storj/satellite/console/consolewasm"
	"storj.io/uplink"
	"storj.io/uplink/private/access"
)

// TestGenerateAccessGrant confirms that the access grant produced by the wasm access code
// is the same as the code the uplink cli uses to create access grants.
func TestGenerateAccessGrant(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		client := newTestClient(t, ctx, planet)
		user := client.defaultUser()
		uplinkPeer := planet.Uplinks[0]
		projectID := uplinkPeer.Projects[0].ID

		client.login(user.email, user.password)

		resp, bodyString := client.request(http.MethodGet, fmt.Sprintf("/projects/%s/salt", projectID.String()), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var b64Salt string
		require.NoError(t, json.Unmarshal([]byte(bodyString), &b64Salt))

		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()

		apiKeyString := uplinkPeer.Projects[0].APIKey

		passphrase := "supersecretpassphrase"

		wasmAccessString, err := console.GenAccessGrant(satelliteNodeURL, apiKeyString, passphrase, b64Salt, true)
		require.NoError(t, err)

		uplinkCliAccess, err := uplinkPeer.Config.RequestAccessWithPassphrase(ctx, satelliteNodeURL, apiKeyString, passphrase)
		require.NoError(t, err)
		uplinkCliAccessString, err := uplinkCliAccess.Serialize()
		require.NoError(t, err)
		require.Equal(t, wasmAccessString, uplinkCliAccessString)

		// test disabled path encryption
		wasmAccessString, err = console.GenAccessGrant(satelliteNodeURL, apiKeyString, passphrase, b64Salt, false)
		require.NoError(t, err)

		access.DisableObjectKeyEncryption(&uplinkPeer.Config)
		uplinkCliAccess, err = uplinkPeer.Config.RequestAccessWithPassphrase(ctx, satelliteNodeURL, apiKeyString, passphrase)
		require.NoError(t, err)
		uplinkCliAccessString, err = uplinkCliAccess.Serialize()
		require.NoError(t, err)
		require.Equal(t, wasmAccessString, uplinkCliAccessString)
	})
}

// TestDefaultAccess confirms that you can perform basic uplink operations with
// the default access grant created from wasm code.
func TestDefaultAccess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellitePeer := planet.Satellites[0]
		satelliteNodeURL := satellitePeer.NodeURL().String()
		uplinkPeer := planet.Uplinks[0]
		APIKey := uplinkPeer.APIKey[satellitePeer.ID()]
		projectID := uplinkPeer.Projects[0].ID
		require.Equal(t, 1, len(uplinkPeer.Projects))

		passphrase := "supersecretpassphrase"
		testbucket1 := "buckettest1"
		testfilename := "file.txt"
		testdata := []byte("fun data")

		client := newTestClient(t, ctx, planet)
		user := client.defaultUser()
		client.login(user.email, user.password)

		resp, bodyString := client.request(http.MethodGet, fmt.Sprintf("/projects/%s/salt", projectID.String()), nil)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var b64Salt string
		require.NoError(t, json.Unmarshal([]byte(bodyString), &b64Salt))

		// Create an access with the console access grant code that allows full access.
		access, err := console.GenAccessGrant(satelliteNodeURL, APIKey.Serialize(), passphrase, b64Salt, true)
		require.NoError(t, err)
		newAccess, err := uplink.ParseAccess(access)
		require.NoError(t, err)
		uplinkPeer.Access[satellitePeer.ID()] = newAccess

		// Confirm that we can create a bucket, upload/download/delete an object, and delete the bucket with the new access.
		require.NoError(t, uplinkPeer.TestingCreateBucket(ctx, satellitePeer, testbucket1))
		err = uplinkPeer.Upload(ctx, satellitePeer, testbucket1, testfilename, testdata)
		require.NoError(t, err)
		data, err := uplinkPeer.Download(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.Equal(t, data, testdata)
		buckets, err := uplinkPeer.ListBuckets(ctx, satellitePeer)
		require.NoError(t, err)
		require.Equal(t, len(buckets), 1)
		err = uplinkPeer.DeleteObject(ctx, satellitePeer, testbucket1, testfilename)
		require.NoError(t, err)
		require.NoError(t, uplinkPeer.DeleteBucket(ctx, satellitePeer, testbucket1))
	})
}

type testClient struct {
	t      *testing.T
	ctx    *testcontext.Context
	planet *testplanet.Planet
	client *http.Client
}

func newTestClient(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) testClient {
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	return testClient{t: t, ctx: ctx, planet: planet, client: &http.Client{Jar: jar}}
}

type registeredUser struct {
	email    string
	password string
}

func (testClient *testClient) request(method string, path string, data io.Reader) (resp Response, body string) {
	req, err := http.NewRequestWithContext(testClient.ctx, method, testClient.url(path), data)
	require.NoError(testClient.t, err)
	req.Header = map[string][]string{
		"sec-ch-ua":        {`" Not A;Brand";v="99"`, `"Chromium";v="90"`, `"Google Chrome";v="90"`},
		"sec-ch-ua-mobile": {"?0"},
		"User-Agent":       {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.93 Safari/537.36"},
		"Content-Type":     {"application/json"},
		"Accept":           {"*/*"},
	}
	return testClient.do(req)
}

// Response is a wrapper for http.Request to prevent false-positive with bodyclose check.
type Response struct{ *http.Response }

func (testClient *testClient) do(req *http.Request) (_ Response, body string) {
	resp, err := testClient.client.Do(req)
	require.NoError(testClient.t, err)

	data, err := io.ReadAll(resp.Body)
	require.NoError(testClient.t, err)
	require.NoError(testClient.t, resp.Body.Close())
	return Response{resp}, string(data)
}

func (testClient *testClient) url(suffix string) string {
	return testClient.planet.Satellites[0].ConsoleURL() + "/api/v0" + suffix
}

func (testClient *testClient) toJSON(v interface{}) io.Reader {
	data, err := json.Marshal(v)
	require.NoError(testClient.t, err)
	return strings.NewReader(string(data))
}

func (testClient *testClient) defaultUser() registeredUser {
	user := testClient.planet.Uplinks[0].User[testClient.planet.Satellites[0].ID()]
	return registeredUser{
		email:    user.Email,
		password: user.Password,
	}
}

func (testClient *testClient) login(email, password string) Response {
	resp, body := testClient.request(
		http.MethodPost, "/auth/token",
		testClient.toJSON(map[string]string{
			"email":    email,
			"password": password,
		}))
	cookie := findCookie(resp, "_tokenKey")
	require.NotNil(testClient.t, cookie)

	var tokenInfo struct {
		Token string `json:"token"`
	}
	require.NoError(testClient.t, json.Unmarshal([]byte(body), &tokenInfo))
	require.Equal(testClient.t, http.StatusOK, resp.StatusCode)
	require.Equal(testClient.t, tokenInfo.Token, cookie.Value)

	return resp
}

func findCookie(response Response, name string) *http.Cookie {
	for _, c := range response.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}
