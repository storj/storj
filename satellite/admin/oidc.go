// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"storj.io/storj/private/api"
)

const (
	oidcSessionCookieName = "admin_session"
	oidcStateCookieName   = "oidc_state"
	oidcPKCECookieName    = "oidc_pkce_verifier"
	oidcStateMaxAge       = 5 * 60 // 5 minutes
)

// ErrOIDC is the error class for OIDC-related errors.
var ErrOIDC = errs.Class("admin oidc")

// OIDCConfig holds configuration for direct OIDC authentication in the admin
// server. When enabled, the admin server handles the full OIDC authorization
// flow itself, removing the need for an external oauth2-proxy.
type OIDCConfig struct {
	Enabled       bool   `help:"whether OIDC auth is enabled" default:"false"`
	ProviderURL   string `help:"OIDC provider URL used for provider discovery"`
	ClientID      string `help:"OIDC client ID"`
	ClientSecret  string `help:"OIDC client secret"`
	GroupsClaim   string `help:"JWT claim name that contains the user's roles or groups" default:"roles"`
	SessionSecret string `help:"secret used to sign session cookies"`
	PKCEEnabled   bool   `help:"whether the OIDC provider supports PKCE" default:"true"`
	LogoutURL     string `help:"Logout URL; used when the provider's discovery document does not include end_session_endpoint"`
}

type sessionClaims struct {
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
	jwt.RegisteredClaims
}

// OIDCHandler handles the OIDC authentication flow for the admin server.
// It must be initialized with Initialize before serving requests.
type OIDCHandler struct {
	log             *zap.Logger
	config          OIDCConfig
	hasEmailRoles   bool
	sessionSecret   []byte
	callbackURL     string
	externalAddress string

	// set in Initialize
	provider           *gooidc.Provider
	verifier           *gooidc.IDTokenVerifier
	oauth2Config       oauth2.Config
	endSessionEndpoint string
}

// NewOIDCHandler creates an OIDCHandler. externalAddress is the public base URL
// of the admin server which will be used to build the OIDC callback address.
func NewOIDCHandler(log *zap.Logger, config OIDCConfig, hasEmailRoles bool, externalAddress string) *OIDCHandler {
	addr := strings.TrimRight(externalAddress, "/")
	return &OIDCHandler{
		log:             log.Named("oidc"),
		config:          config,
		hasEmailRoles:   hasEmailRoles,
		sessionSecret:   []byte(config.SessionSecret),
		externalAddress: addr,
		callbackURL:     addr + "/auth/callback",
	}
}

// Initialize does OIDC provider discovery and sets up the OAuth2 config.
// Without this, OIDCHandler will fail to serve requests.
func (h *OIDCHandler) Initialize(ctx context.Context) (err error) {
	provider, err := gooidc.NewProvider(ctx, h.config.ProviderURL)
	if err != nil {
		return ErrOIDC.Wrap(err)
	}

	h.provider = provider
	h.verifier = provider.Verifier(&gooidc.Config{ClientID: h.config.ClientID})
	h.oauth2Config = oauth2.Config{
		ClientID:     h.config.ClientID,
		ClientSecret: h.config.ClientSecret,
		RedirectURL:  h.callbackURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{gooidc.ScopeOpenID, "email", "profile"},
	}

	var providerClaims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&providerClaims); err != nil {
		return ErrOIDC.Wrap(err)
	}
	rawEndSession := providerClaims.EndSessionEndpoint
	if rawEndSession == "" {
		rawEndSession = h.config.LogoutURL
	}

	if rawEndSession == "" {
		return ErrOIDC.New("OIDC provider does not advertise end_session_endpoint and no LogoutURL is configured")
	}
	endSessionEndpoint, err := url.Parse(rawEndSession)
	if err != nil {
		return ErrOIDC.Wrap(err)
	}

	q := endSessionEndpoint.Query()
	q.Set("client_id", h.config.ClientID)
	q.Set("post_logout_redirect_uri", h.externalAddress+"/auth/login")
	endSessionEndpoint.RawQuery = q.Encode()
	h.endSessionEndpoint = endSessionEndpoint.String()

	return nil
}

// Login redirects to the OIDC provider's authorization
// endpoint. Short-lived state and PKCE verifier cookies are set to prevent
// CSRF attacks and enable PKCE flow.
func (h *OIDCHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err := generateRandomState()
	if err != nil {
		h.httpError(w, http.StatusInternalServerError, "failed to generate OIDC state", err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    state,
		MaxAge:   oidcStateMaxAge,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	var authCodeOpts []oauth2.AuthCodeOption
	if h.config.PKCEEnabled {
		pkceVerifier := oauth2.GenerateVerifier()
		http.SetCookie(w, &http.Cookie{
			Name:     oidcPKCECookieName,
			Value:    pkceVerifier,
			MaxAge:   oidcStateMaxAge,
			HttpOnly: true,
			Secure:   r.TLS != nil,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
		authCodeOpts = append(authCodeOpts, oauth2.S256ChallengeOption(pkceVerifier))
	}

	http.Redirect(w, r, h.oauth2Config.AuthCodeURL(state, authCodeOpts...), http.StatusFound)
}

// Callback exchanges the authorization code from the IdP for tokens,
// validates the ID token, and sets a session cookie on success.
func (h *OIDCHandler) Callback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid or missing state parameter", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:   oidcStateCookieName,
		MaxAge: -1,
		Path:   "/",
	})

	var exchangeOpts []oauth2.AuthCodeOption
	if h.config.PKCEEnabled {
		pkceCookie, err := r.Cookie(oidcPKCECookieName)
		if err != nil {
			h.httpError(w, http.StatusBadRequest, "missing PKCE verifier cookie", err)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:   oidcPKCECookieName,
			MaxAge: -1,
			Path:   "/",
		})
		exchangeOpts = append(exchangeOpts, oauth2.VerifierOption(pkceCookie.Value))
	}

	oauthToken, err := h.oauth2Config.Exchange(r.Context(), r.URL.Query().Get("code"), exchangeOpts...)
	if err != nil {
		h.httpError(w, http.StatusInternalServerError, "OIDC token exchange failed", err)
		return
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		h.httpError(w, http.StatusInternalServerError, "no id_token in OIDC response", nil)
		return
	}

	idToken, err := h.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		h.httpError(w, http.StatusUnauthorized, "OIDC token verification failed", err)
		return
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		h.httpError(w, http.StatusInternalServerError, "failed to parse OIDC claims", err)
		return
	}
	if claims.Email == "" {
		h.httpError(w, http.StatusUnauthorized, "missing email claim in OIDC token", nil)
		return
	}

	groups, err := h.extractGroupsClaim(idToken)
	if err != nil {
		h.httpError(w, http.StatusInternalServerError, "failed to extract groups claim", err)
		return
	}

	if err := h.setSession(w, r, claims.Email, groups, idToken.Expiry); err != nil {
		h.httpError(w, http.StatusInternalServerError, "failed to create admin session", err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

// Logout clears the admin session cookie and redirects to the OIDC provider's
// end_session_endpoint to end the IdP session.
func (h *OIDCHandler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: oidcSessionCookieName, MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookieName, MaxAge: -1, Path: "/"})

	if h.endSessionEndpoint != "" {
		http.Redirect(w, r, h.endSessionEndpoint, http.StatusFound)
		return
	}

	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

// FrontChannelLogout clears the admin session cookie and redirects to /auth/login.
// It is called by the OIDC provider from another application sharing the same IdP session.
func (h *OIDCHandler) FrontChannelLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: oidcSessionCookieName, MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookieName, MaxAge: -1, Path: "/"})
	http.SetCookie(w, &http.Cookie{Name: oidcPKCECookieName, MaxAge: -1, Path: "/"})
	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

// CurrentUser returns the email and groups of the currently authenticated user
// from the session cookie.
func (h *OIDCHandler) CurrentUser(w http.ResponseWriter, r *http.Request) {
	type response struct {
		Email  string   `json:"email"`
		Groups []string `json:"groups"`
	}

	email, groups, err := h.getSession(r)
	if err != nil {
		api.ServeError(h.log, w, http.StatusUnauthorized,
			Error.Wrap(ErrOIDC.New("no active session")))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response{Email: email, Groups: groups}); err != nil {
		h.httpError(w, http.StatusInternalServerError, "failed to encode session info", err)
	}
}

// OIDCMiddleware validates the admin session cookie and injects X-Forwarded-Groups and
// X-Forwarded-Email header for downstream handlers (This is also for
// backward compatibility with our use of Oauth2 Proxy, which also injects this header).
// Unauthenticated API requests receive a 401 response while all other unauthenticated
// requests are redirected to /auth/login (except auth and static routes).
func (h *OIDCHandler) OIDCMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/auth/") || strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}

		email, groups, err := h.getSession(r)
		if err != nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				api.ServeError(h.log, w, http.StatusUnauthorized,
					Error.Wrap(ErrOIDC.New("authentication required")))
				return
			}
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Inject auth info as headers so that Authorizer.GetAuthInfo works
		// without any changes.
		r.Header.Set("X-Forwarded-Groups", strings.Join(groups, ","))
		r.Header.Set("X-Forwarded-Email", email)

		next.ServeHTTP(w, r)
	})
}

func (h *OIDCHandler) setSession(w http.ResponseWriter, r *http.Request, email string, groups []string, expiry time.Time) error {
	claims := sessionClaims{
		Email:  email,
		Groups: groups,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.sessionSecret)
	if err != nil {
		return ErrOIDC.Wrap(err)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcSessionCookieName,
		Value:    signed,
		MaxAge:   int(time.Until(expiry).Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	return nil
}

// getSession validates the admin session cookie, returning the email and groups stored in it.
func (h *OIDCHandler) getSession(r *http.Request) (email string, groups []string, err error) {
	cookie, err := r.Cookie(oidcSessionCookieName)
	if err != nil {
		return "", nil, ErrOIDC.Wrap(err)
	}

	var claims sessionClaims
	_, err = jwt.ParseWithClaims(cookie.Value, &claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrOIDC.New("unexpected signing method: %v", t.Header["alg"])
		}
		return h.sessionSecret, nil
	})
	if err != nil {
		return "", nil, ErrOIDC.Wrap(err)
	}

	return claims.Email, claims.Groups, nil
}

// extractGroupsClaim parses the groups claim from the ID token. Returns an error if no group
// claim is found and no email-role mappings are configured.
func (h *OIDCHandler) extractGroupsClaim(idToken *gooidc.IDToken) ([]string, error) {
	var rawClaims map[string]json.RawMessage
	if err := idToken.Claims(&rawClaims); err != nil {
		return nil, errs.New("failed to parse raw OIDC claims: %w", err)
	}

	raw, ok := rawClaims[h.config.GroupsClaim]
	if !ok {
		if !h.hasEmailRoles {
			return nil, errs.New("no group claim found and no email-role mappings configured")
		}
		return nil, nil
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, errs.New("claim %q is not a string array", h.config.GroupsClaim)
	}
	return arr, nil
}

func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", ErrOIDC.Wrap(err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *OIDCHandler) httpError(w http.ResponseWriter, status int, msg string, err error) {
	if err != nil {
		h.log.Error(msg, zap.Error(err))
	} else {
		h.log.Error(msg)
	}
	http.Error(w, msg, status)
}

// TestSetSession exposes setSession for tests.
func (h *OIDCHandler) TestSetSession(w http.ResponseWriter, r *http.Request, email string, groups []string, expiry time.Time) error {
	return h.setSession(w, r, email, groups, expiry)
}

// TestGetSession exposes getSession for tests.
func (h *OIDCHandler) TestGetSession(r *http.Request) (email string, groups []string, err error) {
	return h.getSession(r)
}
