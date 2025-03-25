// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/go-oauth2/oauth2/v4"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

// UUIDAuthorizeGenerate generates an auth code using Storj's uuid.
type UUIDAuthorizeGenerate struct{}

// Token returns a new authorization code.
func (a *UUIDAuthorizeGenerate) Token(ctx context.Context, data *oauth2.GenerateBasic) (string, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	code, err := uuid.New()
	if err != nil {
		return "", err
	}

	return code.String(), nil
}

// MacaroonAccessGenerate provides an access_token and refresh_token generator using Storj's Macaroons.
type MacaroonAccessGenerate struct {
	Service GenerateService
}

// GenerateService defines the minimal interface needed to generate macaroon based api keys.
type GenerateService interface {
	GetAPIKeyInfoByName(context.Context, uuid.UUID, string) (*console.APIKeyInfo, error)
	CreateAPIKey(context.Context, uuid.UUID, string, macaroon.APIKeyVersion) (*console.APIKeyInfo, *macaroon.APIKey, error)
	GetUser(ctx context.Context, id uuid.UUID) (u *console.User, err error)
}

func (a *MacaroonAccessGenerate) apiKeyForProject(ctx context.Context, data *oauth2.GenerateBasic, project string) (*macaroon.APIKey, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	userID, err := uuid.FromString(data.UserID)
	if err != nil {
		return nil, err
	}

	projectID, err := uuid.FromString(project)
	if err != nil {
		return nil, err
	}

	user, err := a.Service.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	ctx = console.WithUser(ctx, user)

	oauthClient := data.Client.(OAuthClient)
	name := oauthClient.AppName + " / " + oauthClient.ID.String()

	var key *macaroon.APIKey

	apiKeyInfo, err := a.Service.GetAPIKeyInfoByName(ctx, projectID, name)
	if err == nil {
		key, err = macaroon.FromParts(apiKeyInfo.Head, apiKeyInfo.Secret)
	} else if errors.Is(err, sql.ErrNoRows) {
		_, key, err = a.Service.CreateAPIKey(ctx, projectID, name, macaroon.APIKeyVersionMin)
	}

	if err != nil {
		return nil, err
	}

	return key, nil
}

// Token issues access and refresh tokens that are backed by storj's Macaroons. This expects several scopes to be set on
// the request. The following describes the available scopes supported by the macaroon style of access token.
//
//	project:<projectId>  - required, scopes operations to a single project (one)
//	bucket:<name>        - optional, scopes operations to one or many buckets (repeatable)
//	object:list          - optional, allows listing object data
//	object:read          - optional, allows reading object data
//	object:write         - optional, allows writing object data
//	object:delete        - optional, allows deleting object data
//
// In OAuth2.0, access_tokens are short-lived tokens that authorize operations to be performed on behalf of an end user.
// refresh_tokens are longer lived tokens that allow you to obtain new authorization tokens.
func (a *MacaroonAccessGenerate) Token(ctx context.Context, data *oauth2.GenerateBasic, isGenRefresh bool) (access, refresh string, err error) {
	defer mon.Task()(&ctx)(&err)

	var apiKey *macaroon.APIKey

	if priorRefresh := data.TokenInfo.GetRefresh(); isGenRefresh && priorRefresh != "" {
		apiKey, err = macaroon.ParseAPIKey(priorRefresh)
		if err != nil {
			return access, refresh, err
		}

		refresh = priorRefresh
	} else {
		info, perms, err := parseScope(data.TokenInfo.GetScope())
		if err != nil {
			return access, refresh, err
		}

		if info.Project == "" {
			return access, refresh, errors.New("missing project")
		}

		apiKey, err = a.apiKeyForProject(ctx, data, info.Project)
		if err != nil {
			return access, refresh, err
		}

		apiKey, err = apiKey.Restrict(perms)
		if err != nil {
			return access, refresh, err
		}

		if isGenRefresh {
			nonce, err := uuid.New()
			if err != nil {
				return "", "", err
			}

			createAt := data.TokenInfo.GetRefreshCreateAt()
			expireAt := createAt.Add(data.TokenInfo.GetRefreshExpiresIn())

			apiKey, err = apiKey.Restrict(macaroon.Caveat{
				NotBefore: &(createAt),
				NotAfter:  &(expireAt),
				Nonce:     nonce.Bytes(),
			})

			if err != nil {
				return access, refresh, err
			}

			refresh = apiKey.Serialize()
		}
	}

	nonce, err := uuid.New()
	if err != nil {
		return "", "", err
	}

	createAt := data.TokenInfo.GetAccessCreateAt()
	expireAt := createAt.Add(data.TokenInfo.GetAccessExpiresIn())

	apiKey, err = apiKey.Restrict(macaroon.Caveat{
		NotBefore: &(createAt),
		NotAfter:  &(expireAt),
		Nonce:     nonce.Bytes(),
	})

	if err != nil {
		return "", "", err
	}

	access = apiKey.Serialize()
	return access, refresh, nil
}

func parseScope(scope string) (UserInfo, macaroon.Caveat, error) {
	scopes := strings.Split(scope, " ")

	info := UserInfo{}
	perms := macaroon.Caveat{
		DisallowLists:   true,
		DisallowReads:   true,
		DisallowWrites:  true,
		DisallowDeletes: true,
		AllowedPaths:    make([]*macaroon.Caveat_Path, 0, len(scopes)),
	}

	for i := 0; i < len(scopes); i++ {
		scopes[i] = strings.TrimSpace(scopes[i])

		switch {
		case strings.HasPrefix(scopes[i], "project:"):
			if info.Project != "" {
				return info, perms, errors.New("multiple project scopes provided")
			}

			info.Project = strings.TrimPrefix(scopes[i], "project:")
		case strings.HasPrefix(scopes[i], "bucket:"):
			bucket := strings.TrimPrefix(scopes[i], "bucket:")
			info.Buckets = append(info.Buckets, bucket)

			perms.AllowedPaths = append(perms.AllowedPaths, &macaroon.Caveat_Path{
				Bucket: []byte(bucket),
			})
		case strings.HasPrefix(scopes[i], "cubbyhole:"):
			info.Cubbyhole = strings.TrimPrefix(scopes[i], "cubbyhole:")
		case scopes[i] == "object:list":
			perms.DisallowLists = false
		case scopes[i] == "object:read":
			perms.DisallowReads = false
		case scopes[i] == "object:write":
			perms.DisallowWrites = false
		case scopes[i] == "object:delete":
			perms.DisallowDeletes = false
		}
	}

	return info, perms, nil
}
