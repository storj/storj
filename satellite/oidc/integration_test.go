// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/oidc"
	"storj.io/uplink"
)

func send(t *testing.T, body io.Reader, response interface{}, status int, parts ...string) {
	for len(parts) < 4 {
		parts = append(parts, "")
	}

	method := parts[1]
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(t.Context(), method, parts[0], body)
	require.NoError(t, err)

	auth := parts[2]
	if auth != "" {
		req.Header.Set("Authorization", auth)

		req.AddCookie(&http.Cookie{
			Name:     "_tokenKey",
			Value:    auth,
			Path:     "/",
			HttpOnly: true,
		})
	}

	contentType := parts[3]
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, status, resp.StatusCode)

	if response != nil {
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		err = json.Unmarshal(data, response)
		require.NoError(t, err)
	}
}

func TestOIDC(t *testing.T) {
	id, err := uuid.New()
	require.NoError(t, err)

	userID, err := uuid.New()
	require.NoError(t, err)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]

		adminAddr := sat.Admin.Admin.Listener.Addr().String()
		consoleAddr := sat.API.Console.Listener.Addr().String()

		issuer := "http://" + consoleAddr + "/"
		authEndpoint := "http://" + consoleAddr + "/api/v0/oauth/v2/authorize"
		tokenEndpoint := "http://" + consoleAddr + "/api/v0/oauth/v2/tokens"
		userinfoEndpoint := "http://" + consoleAddr + "/api/v0/oauth/v2/userinfo"

		// Setup test user

		regToken, err := sat.API.Console.Service.CreateRegToken(ctx, 1)
		require.NoError(t, err)

		user, err := sat.API.Console.Service.CreateUser(ctx, console.CreateUser{
			FullName: "User",
			Email:    "u@mail.test",
			Password: "password",
		}, regToken.Secret)
		require.NoError(t, err)

		activationToken, err := sat.API.Console.Service.GenerateActivationToken(ctx, user.ID, user.Email)
		require.NoError(t, err)

		user, err = sat.API.Console.Service.ActivateAccount(ctx, activationToken)
		require.NoError(t, err)

		tokenInfo, err := sat.API.Console.Service.GenerateSessionToken(ctx, console.SessionTokenRequest{
			UserID:          user.ID,
			Email:           user.Email,
			IP:              "",
			UserAgent:       "",
			AnonymousID:     "",
			CustomDuration:  nil,
			HubspotObjectID: user.HubspotObjectID,
		})
		require.NoError(t, err)

		// Set up a test project and bucket

		authed := console.WithUser(ctx, user)

		project, err := sat.API.Console.Service.CreateProject(authed, console.UpsertProjectInfo{
			Name: "test",
		})
		require.NoError(t, err)

		bucketID, err := uuid.New()
		require.NoError(t, err)

		bucket, err := sat.API.Buckets.Service.CreateBucket(authed, buckets.Bucket{
			ID:        bucketID,
			Name:      "test",
			ProjectID: project.ID,
		})
		require.NoError(t, err)

		// Create a client that will receive our tokens

		callback, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer func() { _ = callback.Close() }()

		client := oidc.OAuthClient{
			ID:          id,
			Secret:      []byte("badadmin"),
			UserID:      userID,
			RedirectURL: "http://" + callback.Addr().String(),
		}

		adminClients := fmt.Sprintf("http://%s/api/oauth/clients", adminAddr)

		{
			body, err := json.Marshal(client)
			require.NoError(t, err)

			send(t, bytes.NewReader(body), nil, http.StatusOK, adminClients, http.MethodPost, sat.Config.Console.AuthToken)
		}

		// Ensure OpenID Connect's well-known configuration endpoint works.

		wellKnownConfig := fmt.Sprintf("http://%s/api/v0/.well-known/openid-configuration", consoleAddr)

		cfg := oidc.ProviderConfig{}
		send(t, nil, &cfg, http.StatusOK, wellKnownConfig)

		require.Equal(t, issuer, cfg.Issuer)
		require.Equal(t, authEndpoint, cfg.AuthURL)
		require.Equal(t, tokenEndpoint, cfg.TokenURL)
		require.Equal(t, userinfoEndpoint, cfg.UserInfoURL)

		// While we don't register a GET handler on the server, we need to ensure that the server returns in a 200
		// request. This effectively delegates handling of the route to the Vue controller in the browser. If the
		// server issues a redirect, we drop the encryption key in the fragment making it impossible for the client
		// to encrypt the derived encryption key.
		send(t, nil, nil, http.StatusOK, authEndpoint+"#fake-encryption-key")

		// Prepare exchange for token

		token := oauth2.Token{}
		oauth2Config := oauth2.Config{
			ClientID:     client.ID.String(),
			ClientSecret: string(client.Secret),
			RedirectURL:  client.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  authEndpoint,
				TokenURL: tokenEndpoint,
			},
			Scopes: []string{
				"openid",
				"object:read",
				"object:write",
				"object:delete",
			},
		}

		state, err := uuid.New()
		require.NoError(t, err)

		// Set up the callback server to receive our single-use code and exchange for our initial set of tokens.

		server := &http.Server{}
		defer func() { _ = server.Shutdown(ctx) }()

		server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			code := q.Get("code")

			require.Equal(t, state.String(), q.Get("state"))
			require.NotEqual(t, "", code)

			token, err := oauth2Config.Exchange(r.Context(), code)
			require.NoError(t, err)

			err = json.NewEncoder(w).Encode(token)
			require.NoError(t, err)
		})

		group := errgroup.Group{}
		group.Go(func() error {
			return server.Serve(callback)
		})

		// Mock submitting the consent screen, granting the application the following permissions.

		scope := fmt.Sprintf("project:%s bucket:%s cubbyhole:cyphertext object:list object:read object:write object:delete",
			project.ID.String(), bucket.Name)

		consent := url.Values{}
		consent.Set("redirect_uri", client.RedirectURL)
		consent.Set("client_id", client.ID.String())
		consent.Set("response_type", "code")
		consent.Set("state", state.String())
		consent.Set("scope", scope)

		{
			body := strings.NewReader(consent.Encode())
			send(t, body, &token, http.StatusOK, authEndpoint, http.MethodPost, tokenInfo.Token.String(), "application/x-www-form-urlencoded")
		}

		require.Equal(t, "Bearer", token.TokenType)
		require.NotEqual(t, "", token.AccessToken)
		require.NotEqual(t, "", token.RefreshToken)

		// Refresh the tokens.

		refresh := url.Values{}
		refresh.Set("grant_type", "refresh_token")
		refresh.Set("refresh_token", token.RefreshToken)

		refreshed := oauth2.Token{}
		auth := base64.StdEncoding.EncodeToString([]byte(client.ID.String() + ":" + string(client.Secret)))

		{
			body := strings.NewReader(refresh.Encode())
			send(t, body, &refreshed, http.StatusOK, tokenEndpoint, http.MethodPost, "Basic "+auth, "application/x-www-form-urlencoded")
		}

		require.Equal(t, token.RefreshToken, refreshed.RefreshToken)
		require.NotEqual(t, token.AccessToken, refreshed.AccessToken)

		// Fetch UserInfo

		info := oidc.UserInfo{}
		send(t, nil, &info, http.StatusOK, userinfoEndpoint, http.MethodGet, "Bearer "+token.AccessToken)

		require.Equal(t, "cyphertext", info.Cubbyhole)

		// Use token with uplink

		apiKey, err := macaroon.ParseAPIKey(token.AccessToken)
		require.NoError(t, err)

		// in practice, you should decrypt the cubbyhole and pass it here
		key, err := storj.NewKey([]byte(info.Cubbyhole))
		require.NoError(t, err)

		encAccess := grant.NewEncryptionAccessWithDefaultKey(key)
		encAccess.SetDefaultKey(key)
		encAccess.SetDefaultPathCipher(storj.EncAESGCM)

		accessGrant, err := (&grant.Access{
			SatelliteAddress: sat.NodeURL().String(),
			APIKey:           apiKey,
			EncAccess:        encAccess,
		}).Serialize()

		require.NoError(t, err)

		access, err := uplink.ParseAccess(accessGrant)
		require.NoError(t, err)

		proj, err := uplink.OpenProject(ctx, access)
		require.NoError(t, err)

		upload, err := proj.UploadObject(ctx, bucket.Name, "testing/1/2/3", &uplink.UploadOptions{})
		require.NoError(t, err)

		defer func() { _ = upload.Abort() }()

		_, err = upload.Write([]byte("hello world!"))
		require.NoError(t, err)

		err = upload.Commit()
		require.NoError(t, err)

		download, err := proj.DownloadObject(ctx, bucket.Name, "testing/1/2/3", &uplink.DownloadOptions{
			Length: -1,
		})
		require.NoError(t, err)

		defer func() { _ = download.Close() }()

		content, err := io.ReadAll(download)
		require.NoError(t, err)

		require.Equal(t, "hello world!", string(content))
	})
}
