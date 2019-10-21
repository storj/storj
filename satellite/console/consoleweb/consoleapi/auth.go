// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"storj.io/storj/internal/post"
	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
)

// Auth is an api controller that exposes all auth functionality.
type Auth struct {
	log         *zap.Logger
	service     *console.Service
	mailService *mailservice.Service

	ExternalAddress string
}

// NewAuth is a constructor for api auth controller.
func NewAuth(log *zap.Logger, service *console.Service, mailService *mailservice.Service, externalAddress string) *Auth {
	return &Auth{
		log:             log,
		service:         service,
		mailService:     mailService,
		ExternalAddress: externalAddress,
	}
}

// Token authenticates User by credentials and returns auth token.
func (a *Auth) Token(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var tokenRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	err = json.NewDecoder(r.Body).Decode(&tokenRequest)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	var tokenResponse struct {
		Token string `json:"token"`
	}

	tokenResponse.Token, err = a.service.Token(ctx, tokenRequest.Email, tokenRequest.Password)
	if err != nil {
		a.serveJSONError(w, http.StatusUnauthorized, err)
		return
	}

	err = json.NewEncoder(w).Encode(tokenResponse)
	if err != nil {
		a.log.Error("token handler could not encode token response", zap.Error(err))
		return
	}
}

// Register creates new User, sends activation e-mail.
func (a *Auth) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var request struct {
		UserInfo       console.CreateUser `json:"userInfo"`
		SecretInput    string             `json:"secret"`
		ReferrerUserID string             `json:"referrerUserID"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	secret, err := console.RegistrationSecretFromBase64(request.SecretInput)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	user, err := a.service.CreateUser(ctx, request.UserInfo, secret, request.ReferrerUserID)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}

	token, err := a.service.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}

	link := a.ExternalAddress + consoleql.ActivationPath + token
	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	a.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.AccountActivationEmail{
			ActivationLink: link,
			Origin:         a.ExternalAddress,
		},
	)

	err = json.NewEncoder(w).Encode(&user.ID)
	if err != nil {
		a.log.Error("registration handler could not encode error", zap.Error(err))
		return
	}
}

// PasswordChange auth user, changes users password for a new one.
func (a *Auth) PasswordChange(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var passwordChange struct {
		CurrentPassword string `json:"password"`
		NewPassword     string `json:"newPassword"`
	}

	ctx = a.authorize(ctx, r)

	err = json.NewDecoder(r.Body).Decode(&passwordChange)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	err = a.service.ChangePassword(ctx, passwordChange.CurrentPassword, passwordChange.NewPassword)
	if err != nil {
		a.serveJSONError(w, http.StatusNotFound, err)
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (a *Auth) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		a.log.Error("failed to write json error response", zap.Error(err))
	}
}

// authorize checks request for authorization token, validates it and updates context with auth data.
func (a *Auth) authorize(ctx context.Context, r *http.Request) context.Context {
	authHeaderValue := r.Header.Get("Authorization")
	token := strings.TrimPrefix(authHeaderValue, "Bearer ")

	auth, err := a.service.Authorize(auth.WithAPIKey(ctx, []byte(token)))
	if err != nil {
		return console.WithAuthFailure(ctx, err)
	}

	return console.WithAuth(ctx, auth)
}
