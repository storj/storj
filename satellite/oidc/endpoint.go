// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-oauth2/oauth2/v4"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

var (
	mon = monkit.Package()
)

// NewEndpoint constructs an OpenID identity provider.
func NewEndpoint(
	nodeURL storj.NodeURL, externalAddress string, log *zap.Logger,
	oidcService *Service, service *console.Service,
	codeExpiry, accessTokenExpiry, refreshTokenExpiry time.Duration,
) *Endpoint {
	manager := manage.NewManager()

	clientStore := oidcService.ClientStore()
	tokenStore := oidcService.TokenStore()

	manager.MapClientStorage(clientStore)
	manager.MapTokenStorage(tokenStore)

	manager.MapAuthorizeGenerate(&UUIDAuthorizeGenerate{})
	manager.SetAuthorizeCodeExp(codeExpiry)

	manager.MapAccessGenerate(&MacaroonAccessGenerate{Service: service})
	manager.SetRefreshTokenCfg(&manage.RefreshingConfig{
		AccessTokenExp:    accessTokenExpiry,
		RefreshTokenExp:   refreshTokenExpiry,
		IsGenerateRefresh: refreshTokenExpiry > 0,
	})

	svr := server.NewDefaultServer(manager)

	svr.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (userID string, err error) {
		user, err := console.GetUser(r.Context())
		if err != nil {
			return "", console.ErrUnauthorized.Wrap(err)
		}

		return user.ID.String(), nil
	})

	// externalAddress _should_ end with a '/' suffix based on the calling path
	return &Endpoint{
		clientStore: clientStore,
		tokenStore:  tokenStore,
		service:     service,
		server:      svr,
		log:         log,
		config: ProviderConfig{
			NodeURL:     nodeURL.String(),
			Issuer:      externalAddress,
			AuthURL:     externalAddress + "api/v0/oauth/v2/authorize",
			TokenURL:    externalAddress + "api/v0/oauth/v2/tokens",
			UserInfoURL: externalAddress + "api/v0/oauth/v2/userinfo",
		},
	}
}

// Endpoint implements an OpenID Connect (OIDC) Identity Provider. It grants client applications access to resources
// in the Storj network on behalf of the end user.
//
// architecture: Endpoint
type Endpoint struct {
	clientStore oauth2.ClientStore
	tokenStore  oauth2.TokenStore
	service     *console.Service
	server      *server.Server
	log         *zap.Logger
	config      ProviderConfig
}

// WellKnownConfiguration renders the identity provider configuration that points clients to various endpoints.
func (e *Endpoint) WellKnownConfiguration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(e.config)
	if err != nil {
		e.log.Error("failed to encode oidc config", zap.Error(err))
	}
}

// AuthorizeUser is called from an authenticated context granting the requester access to the application. We redirect
// back to the client application with the provided state and obtained code.
func (e *Endpoint) AuthorizeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	err = e.server.HandleAuthorizeRequest(w, r)
	if err != nil {
		e.log.Error("failed to authorize user", zap.Error(err))
	}
}

// Tokens exchanges unexpired refresh tokens or codes provided by AuthorizeUser for the associated set of tokens.
func (e *Endpoint) Tokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	err = e.server.HandleTokenRequest(w, r)
	if err != nil {
		e.log.Error("failed to exchange for token", zap.Error(err))
	}
}

// UserInfo uses the provided access token to look up the associated user information.
func (e *Endpoint) UserInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	accessToken := r.Header.Get("Authorization")
	if !strings.HasPrefix(accessToken, "Bearer ") {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	accessToken = strings.TrimPrefix(accessToken, "Bearer ")

	info, err := e.tokenStore.GetByAccess(ctx, accessToken)
	if err != nil || info == nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	userInfo, _, err := parseScope(info.GetScope())
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.FromString(info.GetUserID())
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	user, err := e.service.GetUser(ctx, userID)
	if err != nil {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	if user.Status != console.Active {
		http.Error(w, "", http.StatusUnauthorized)
		return
	}

	userInfo.Subject = user.ID
	userInfo.Email = user.Email
	userInfo.EmailVerified = true

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(userInfo)
	if err != nil {
		e.log.Error("failed to encode user info", zap.Error(err))
	}
}

// GetClient returns non-sensitive information about an OAuthClient. This information is used to initially verify client
// applications who are requesting information on behalf of a user.
func (e *Endpoint) GetClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	vars := mux.Vars(r)

	client, err := e.clientStore.GetByID(ctx, vars["id"])
	switch {
	case errors.Is(err, sql.ErrNoRows):
		http.NotFound(w, r)
		return
	case err != nil:
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	err = json.NewEncoder(w).Encode(client)
	if err != nil {
		e.log.Error("failed to encode oauth client", zap.Error(err))
	}
}

// ProviderConfig defines a subset of elements used by OIDC to auto-discover endpoints.
type ProviderConfig struct {
	NodeURL     string `json:"node_url"`
	Issuer      string `json:"issuer"`
	AuthURL     string `json:"authorization_endpoint"`
	TokenURL    string `json:"token_endpoint"`
	UserInfoURL string `json:"userinfo_endpoint"`
}

// UserInfo provides a semi-standard object for common user information. The "cubbyhole" value is used to share the
// derived encryption key between client applications. In order to obtain it, the requesting client must decrypt
// the value using the key they provided when redirecting the user to login.
type UserInfo struct {
	Subject       uuid.UUID `json:"sub"`
	Email         string    `json:"email"`
	EmailVerified bool      `json:"email_verified"`

	// custom values below

	Project   string   `json:"project"`
	Buckets   []string `json:"buckets"`
	Cubbyhole string   `json:"cubbyhole"`
}
