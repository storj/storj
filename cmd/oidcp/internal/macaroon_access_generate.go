package internal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/go-oauth2/oauth2/v4"
	"storj.io/common/macaroon"
	"storj.io/common/uuid"

	"storj.io/storj/satellite/console"
)

// MacaroonAccessGenerate provides an access_token and refresh_token generator using Storj's Macaroons.
type MacaroonAccessGenerate struct {
	DB console.DB
}

func (a *MacaroonAccessGenerate) secretForProject(ctx context.Context, data *oauth2.GenerateBasic, project string) ([]byte, error) {
	projectUUID, err := uuid.FromString(project)
	if err != nil {
		return nil, err
	}

	apiKeyInfo, err := a.DB.APIKeys().GetByNameAndProjectID(ctx, data.Client.GetID(), projectUUID)
	if errors.Is(err, sql.ErrNoRows) {
		secret, err := macaroon.NewSecret()
		if err != nil {
			return nil, err
		}

		apiKey, err := macaroon.NewAPIKey(secret)
		if err != nil {
			return nil, err
		}

		apiKeyInfo, err = a.DB.APIKeys().Create(ctx, apiKey.Head(), console.APIKeyInfo{
			Name:      data.Client.GetID(),
			ProjectID: projectUUID,
			Secret:    secret,
		})
	} else if err != nil {
		return nil, err
	}

	return apiKeyInfo.Secret, nil
}

// Token issues access and refresh tokens that are backed by storj's Macaroons. This expects several scopes to be set on
// the request. The following describes the available scopes supported by the macaroon style of access token.
//
//    project:<projectId>  - required, scopes operations to a single project (one)
//    bucket:<name>        - optional, scopes operations to one or many buckets (repeatable)
//    object:list          - optional, allows listing object data
//    object:read          - optional, allows reading object data
//    object:write         - optional, allows writing object data
//    object:delete        - optional, allows deleting object data
//
// In OAuth2.0, access_tokens are short-lived tokens that authorize operations to be performed on behalf of an end user.
// refresh_tokens are longer lived tokens that allow you to obtain new authorization tokens.
func (a *MacaroonAccessGenerate) Token(ctx context.Context, data *oauth2.GenerateBasic, isGenRefresh bool) (access, refresh string, err error) {
	var apiKey *macaroon.APIKey

	// todo: associate apiKey / refresh key to user

	if priorRefresh := data.TokenInfo.GetRefresh(); isGenRefresh && priorRefresh != "" {
		apiKey, err = macaroon.ParseAPIKey(priorRefresh)
		if err != nil {
			return access, refresh, err
		}

		refresh = priorRefresh
	} else {
		// otherwise, mint a new
		scopes := strings.Split(data.TokenInfo.GetScope(), " ")

		project := ""
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
				if project != "" {
					return access, refresh, fmt.Errorf("multiple project scopes provided")
				}

				project = strings.TrimPrefix(scopes[i], "project:")
			case strings.HasPrefix(scopes[i], "bucket:"):
				perms.AllowedPaths = append(perms.AllowedPaths, &macaroon.Caveat_Path{
					Bucket: []byte(strings.TrimPrefix(scopes[i], "bucket:")),
				})
			case scopes[i] == "list":
				perms.DisallowLists = false
			case scopes[i] == "read":
				perms.DisallowReads = false
			case scopes[i] == "write":
				perms.DisallowWrites = false
			case scopes[i] == "delete":
				perms.DisallowDeletes = false
			}
		}

		if project == "" {
			return access, refresh, fmt.Errorf("missing project")
		}

		secret, err := a.secretForProject(ctx, data, project)
		if err != nil {
			return access, refresh, err
		}

		apiKey, err = macaroon.NewAPIKey(secret)
		if err != nil {
			return access, refresh, err
		}

		apiKey, err = apiKey.Restrict(perms)
		if err != nil {
			return access, refresh, err
		}

		if isGenRefresh {
			createAt := data.TokenInfo.GetRefreshCreateAt()
			expireAt := createAt.Add(data.TokenInfo.GetRefreshExpiresIn())

			apiKey, err = apiKey.Restrict(macaroon.Caveat{
				NotBefore: &(createAt),
				NotAfter:  &(expireAt),
			})

			if err != nil {
				return access, refresh, err
			}

			refresh = apiKey.Serialize()
		}
	}

	createAt := data.TokenInfo.GetAccessCreateAt()
	expireAt := createAt.Add(data.TokenInfo.GetAccessExpiresIn())

	apiKey, err = apiKey.Restrict(macaroon.Caveat{
		NotBefore: &(createAt),
		NotAfter:  &(expireAt),
	})

	if err != nil {
		return "", "", err
	}

	access = apiKey.Serialize()
	return access, refresh, nil
}

var _ oauth2.AccessGenerate = &MacaroonAccessGenerate{}
