// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

import (
	"context"
	"time"

	"github.com/go-oauth2/oauth2/v4"

	"storj.io/common/uuid"
)

// ClientStore provides a simple adapter for the oauth implementation.
type ClientStore struct {
	clients OAuthClients
}

var _ oauth2.ClientStore = (*ClientStore)(nil)

// GetByID returns client information by id.
func (c *ClientStore) GetByID(ctx context.Context, id string) (_ oauth2.ClientInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	uid, err := uuid.FromString(id)
	if err != nil {
		return nil, err
	}

	return c.clients.Get(ctx, uid)
}

// TokenStore provides a simple adapter for the oauth implementation.
type TokenStore struct {
	codes  OAuthCodes
	tokens OAuthTokens
}

var _ oauth2.TokenStore = (*TokenStore)(nil)

// Create creates a new token with the given info.
func (t *TokenStore) Create(ctx context.Context, info oauth2.TokenInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	var code OAuthCode
	var access, refresh OAuthToken

	if r, ok := info.(*record); ok {
		code = r.code
		access = r.access
		refresh = r.refresh
	} else {
		clientID, err := uuid.FromString(info.GetClientID())
		if err != nil {
			return err
		}

		userID, err := uuid.FromString(info.GetUserID())
		if err != nil {
			return err
		}

		if c := info.GetCode(); c != "" {
			code.ClientID = clientID
			code.UserID = userID
			code.Scope = info.GetScope()
			code.RedirectURL = info.GetRedirectURI()
			code.Challenge = info.GetCodeChallenge()
			code.ChallengeMethod = string(info.GetCodeChallengeMethod())
			code.Code = c
			code.CreatedAt = info.GetCodeCreateAt()
			code.ExpiresAt = code.CreatedAt.Add(info.GetCodeExpiresIn())
		}

		if a := info.GetAccess(); a != "" {
			access.ClientID = clientID
			access.UserID = userID
			access.Scope = info.GetScope()
			access.Kind = KindAccessToken
			access.Token = a
			access.CreatedAt = info.GetAccessCreateAt()
			access.ExpiresAt = access.CreatedAt.Add(info.GetAccessExpiresIn())
		}

		if r := info.GetRefresh(); r != "" {
			refresh.ClientID = clientID
			refresh.UserID = userID
			refresh.Scope = info.GetScope()
			refresh.Kind = KindRefreshToken
			refresh.Token = r
			refresh.CreatedAt = info.GetRefreshCreateAt()
			refresh.ExpiresAt = refresh.CreatedAt.Add(info.GetRefreshExpiresIn())
		}
	}

	if code.Code != "" {
		err := t.codes.Create(ctx, code)
		if err != nil {
			return err
		}
	}

	if access.Token != "" {
		err := t.tokens.Create(ctx, access)
		if err != nil {
			return err
		}
	}

	if refresh.Token != "" {
		err := t.tokens.Create(ctx, refresh)
		if err != nil {
			return err
		}
	}

	return nil
}

// RemoveByCode deletes token by authorization code.
func (t *TokenStore) RemoveByCode(ctx context.Context, code string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return t.codes.Claim(ctx, code)
}

// RemoveByAccess deletes token by access token.
func (t *TokenStore) RemoveByAccess(ctx context.Context, access string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return nil // unsupported by current configuration
}

// RemoveByRefresh deletes token by refresh token.
func (t *TokenStore) RemoveByRefresh(ctx context.Context, refresh string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return nil // unsupported by current configuration
}

// GetByCode uses authorization code to find token information.
func (t *TokenStore) GetByCode(ctx context.Context, code string) (_ oauth2.TokenInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	oauthCode, err := t.codes.Get(ctx, code)
	if err != nil {
		return nil, err
	}

	return &record{code: oauthCode}, nil
}

// GetByAccess uses access token to find token information.
func (t *TokenStore) GetByAccess(ctx context.Context, access string) (_ oauth2.TokenInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	oauthToken, err := t.tokens.Get(ctx, KindAccessToken, access)
	if err != nil {
		return nil, err
	}

	return &record{access: oauthToken}, nil
}

// GetByRefresh uses refresh token to find token information.
func (t *TokenStore) GetByRefresh(ctx context.Context, refresh string) (_ oauth2.TokenInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	oauthToken, err := t.tokens.Get(ctx, KindRefreshToken, refresh)
	if err != nil {
		return nil, err
	}

	return &record{refresh: oauthToken}, nil
}

type record struct {
	code    OAuthCode
	access  OAuthToken
	refresh OAuthToken
}

func (r *record) New() oauth2.TokenInfo {
	return &record{}
}

func (r *record) GetClientID() string {
	switch {
	case !r.code.ClientID.IsZero():
		return r.code.ClientID.String()
	case !r.access.ClientID.IsZero():
		return r.access.ClientID.String()
	case !r.refresh.ClientID.IsZero():
		return r.refresh.ClientID.String()
	}

	return ""
}

func (r *record) SetClientID(s string) {
	clientID, err := uuid.FromString(s)
	if err != nil {
		return
	}

	r.code.ClientID = clientID
	r.access.ClientID = clientID
	r.refresh.ClientID = clientID
}

func (r *record) GetUserID() string {
	switch {
	case !r.code.UserID.IsZero():
		return r.code.UserID.String()
	case !r.access.UserID.IsZero():
		return r.access.UserID.String()
	case !r.refresh.UserID.IsZero():
		return r.refresh.UserID.String()
	}

	return ""
}

func (r *record) SetUserID(s string) {
	userID, err := uuid.FromString(s)
	if err != nil {
		return
	}

	r.code.ClientID = userID
	r.access.ClientID = userID
	r.refresh.ClientID = userID
}

func (r *record) GetScope() string {
	switch {
	case r.code.Scope != "":
		return r.code.Scope
	case r.access.Scope != "":
		return r.access.Scope
	case r.refresh.Scope != "":
		return r.refresh.Scope
	}

	return ""
}

func (r *record) SetScope(scope string) {
	r.code.Scope = scope
	r.access.Scope = scope
	r.refresh.Scope = scope
}

func (r *record) GetRedirectURI() string {
	return r.code.RedirectURL
}

func (r *record) SetRedirectURI(redirectURL string) {
	r.code.RedirectURL = redirectURL
}

func (r *record) GetCode() string {
	return r.code.Code
}

func (r *record) SetCode(code string) {
	r.code.Code = code
}

func (r *record) GetCodeCreateAt() time.Time {
	return r.code.CreatedAt
}

func (r *record) SetCodeCreateAt(time time.Time) {
	r.code.CreatedAt = time
}

func (r *record) GetCodeExpiresIn() time.Duration {
	return r.code.ExpiresAt.Sub(r.code.CreatedAt)
}

func (r *record) SetCodeExpiresIn(duration time.Duration) {
	r.code.ExpiresAt = r.code.CreatedAt.Add(duration)
}

func (r *record) GetCodeChallenge() string {
	return r.code.Challenge
}

func (r *record) SetCodeChallenge(challenge string) {
	r.code.Challenge = challenge
}

func (r *record) GetCodeChallengeMethod() oauth2.CodeChallengeMethod {
	if r.code.ChallengeMethod == string(oauth2.CodeChallengeS256) {
		return oauth2.CodeChallengeS256
	}

	return oauth2.CodeChallengePlain
}

func (r *record) SetCodeChallengeMethod(method oauth2.CodeChallengeMethod) {
	r.code.ChallengeMethod = string(method)
}

func (r *record) GetAccess() string {
	return r.access.Token
}

func (r *record) SetAccess(token string) {
	r.access.Token = token
}

func (r *record) GetAccessCreateAt() time.Time {
	return r.access.CreatedAt
}

func (r *record) SetAccessCreateAt(time time.Time) {
	r.access.CreatedAt = time
}

func (r *record) GetAccessExpiresIn() time.Duration {
	return r.access.ExpiresAt.Sub(r.access.CreatedAt)
}

func (r *record) SetAccessExpiresIn(duration time.Duration) {
	r.access.ExpiresAt = r.access.CreatedAt.Add(duration)
}

func (r *record) GetRefresh() string {
	return r.refresh.Token
}

func (r *record) SetRefresh(token string) {
	r.refresh.Token = token
}

func (r *record) GetRefreshCreateAt() time.Time {
	return r.refresh.CreatedAt
}

func (r *record) SetRefreshCreateAt(time time.Time) {
	r.refresh.CreatedAt = time
}

func (r *record) GetRefreshExpiresIn() time.Duration {
	return r.refresh.ExpiresAt.Sub(r.refresh.CreatedAt)
}

func (r *record) SetRefreshExpiresIn(duration time.Duration) {
	r.refresh.ExpiresAt = r.refresh.CreatedAt.Add(duration)
}
