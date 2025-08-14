// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/models"
	"github.com/stretchr/testify/require"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/oidc"
)

type mockGenerateService struct {
	GetAPIKeyInfoFunc func(ctx context.Context, uuid uuid.UUID, name string) (*console.APIKeyInfo, error)
	CreateAPIKeyFunc  func(ctx context.Context, uuid uuid.UUID, name string, version macaroon.APIKeyVersion) (*console.APIKeyInfo, *macaroon.APIKey, error)
	GetUserFunc       func(ctx context.Context, uuid uuid.UUID) (*console.User, error)
}

func (m *mockGenerateService) GetAPIKeyInfoByName(ctx context.Context, projectID uuid.UUID, name string) (*console.APIKeyInfo, error) {
	if m.GetAPIKeyInfoFunc == nil {
		return nil, nil
	}

	return m.GetAPIKeyInfoFunc(ctx, projectID, name)
}

func (m *mockGenerateService) GetUser(ctx context.Context, id uuid.UUID) (u *console.User, err error) {
	if m.GetUserFunc == nil {
		return nil, nil
	}

	return m.GetUserFunc(ctx, id)
}

func (m *mockGenerateService) CreateAPIKey(ctx context.Context, id uuid.UUID, name string, version macaroon.APIKeyVersion) (*console.APIKeyInfo, *macaroon.APIKey, error) {
	if m.CreateAPIKeyFunc == nil {
		return nil, nil, nil
	}

	return m.CreateAPIKeyFunc(ctx, id, name, version)
}

var _ oidc.GenerateService = &mockGenerateService{}

func TestUUIDGenerate(t *testing.T) {
	ctx := t.Context()

	generate := oidc.UUIDAuthorizeGenerate{}
	uuid, err := generate.Token(ctx, nil)
	require.NoError(t, err)
	require.NotEqual(t, "", uuid)
}

func TestMacaroonGenerate(t *testing.T) {
	secret, err := macaroon.NewSecret()
	require.NoError(t, err)

	apiKey, err := macaroon.NewAPIKey(secret)
	require.NoError(t, err)

	getSuccess := func(ctx context.Context, uuid uuid.UUID, name string) (*console.APIKeyInfo, error) {
		return &console.APIKeyInfo{
			ID:        uuid,
			ProjectID: uuid,
			Name:      name,
			Head:      apiKey.Head(),
			Secret:    secret,
		}, nil
	}

	getFailure := func(ctx context.Context, uuid uuid.UUID, name string) (*console.APIKeyInfo, error) {
		return nil, sql.ErrNoRows
	}

	createSuccess := func(ctx context.Context, uuid uuid.UUID, name string, version macaroon.APIKeyVersion) (*console.APIKeyInfo, *macaroon.APIKey, error) {
		return &console.APIKeyInfo{
			ID:        uuid,
			ProjectID: uuid,
			Name:      name,
			Head:      apiKey.Head(),
			Secret:    secret,
			Version:   version,
		}, apiKey, nil
	}

	user, err := uuid.New()
	require.NoError(t, err)

	project, err := uuid.New()
	require.NoError(t, err)

	missingProjectScope := `object:list object:read object:write object:delete`
	fullScope := "project:" + project.String() + " bucket:test cubbyhole:plaintext " + missingProjectScope
	multipleProjectScopes := "project:" + project.String() + " " + fullScope

	testCases := []struct {
		name    string
		scope   string
		get     func(ctx context.Context, uuid uuid.UUID, name string) (*console.APIKeyInfo, error)
		create  func(ctx context.Context, uuid uuid.UUID, name string, version macaroon.APIKeyVersion) (*console.APIKeyInfo, *macaroon.APIKey, error)
		refresh bool
		err     string
	}{
		{"missing project", missingProjectScope, getSuccess, nil, false, "missing project"},
		{"multiple projects", multipleProjectScopes, getSuccess, nil, false, "multiple project scopes provided"},
		{"create secret - access", fullScope, getFailure, createSuccess, false, ""},
		{"create secret - access and refresh", fullScope, getFailure, createSuccess, true, ""},
		{"existing secret - access", fullScope, getSuccess, nil, false, ""},
		{"existing secret - access and refresh", fullScope, getSuccess, nil, true, ""},
	}

	ctx := t.Context()
	mock := &mockGenerateService{
		GetUserFunc: func(ctx context.Context, uuid uuid.UUID) (*console.User, error) {
			return &console.User{
				ID: user,
			}, nil
		},
	}
	generate := &oidc.MacaroonAccessGenerate{Service: mock}

	token := &models.Token{
		AccessCreateAt:   time.Now(),
		AccessExpiresIn:  time.Minute,
		RefreshCreateAt:  time.Now(),
		RefreshExpiresIn: time.Minute,
	}

	request := &oauth2.GenerateBasic{
		Client:    oidc.OAuthClient{},
		UserID:    user.String(),
		TokenInfo: token,
	}

	for _, testCase := range testCases {
		t.Log(testCase.name)

		token.Refresh = ""
		token.Scope = testCase.scope

		mock.GetAPIKeyInfoFunc = testCase.get
		mock.CreateAPIKeyFunc = testCase.create

		// initial generation
		access, refresh, err := generate.Token(ctx, request, testCase.refresh)
		if testCase.err != "" {
			require.Error(t, err)
			require.Equal(t, testCase.err, err.Error())
			continue
		}

		require.NoError(t, err)
		require.NotEqual(t, "", access)

		if !testCase.refresh {
			require.Equal(t, "", refresh)
			continue
		}

		require.NotEqual(t, "", refresh)

		// test regeneration
		token.Refresh = refresh
		refreshed, refresh, err := generate.Token(ctx, request, testCase.refresh)

		require.NoError(t, err)
		require.Equal(t, token.Refresh, refresh)

		// ensure the refreshed token isn't the same as the original
		require.NotEqual(t, access, refreshed)
	}
}
