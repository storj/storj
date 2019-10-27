// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/internal/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
)

// ErrAuthAPI - console auth api error type.
var ErrAuthAPI = errs.Class("console auth api error")

// Auth is an api controller that exposes all auth functionality.
type Auth struct {
	log                   *zap.Logger
	service               *console.Service
	mailService           *mailservice.Service
	ExternalAddress       string
	LetUsKnowURL          string
	TermsAndConditionsURL string
	ContactInfoURL        string
}

// NewAuth is a constructor for api auth controller.
func NewAuth(log *zap.Logger, service *console.Service, mailService *mailservice.Service, externalAddress string, letUsKnowURL string, termsAndConditionsURL string, contactInfoURL string) *Auth {
	return &Auth{
		log:                   log,
		service:               service,
		mailService:           mailService,
		ExternalAddress:       externalAddress,
		LetUsKnowURL:          letUsKnowURL,
		TermsAndConditionsURL: termsAndConditionsURL,
		ContactInfoURL:        contactInfoURL,
	}
}

// Token authenticates user by credentials and returns auth token.
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
		a.log.Error("token handler could not encode token response", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// Register creates new user, sends activation e-mail.
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
		a.log.Error("registration handler could not encode error", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// Update updates user's full name and short name.
func (a *Auth) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var updatedInfo struct {
		FullName  string `json:"fullName"`
		ShortName string `json:"shortName"`
	}

	err = json.NewDecoder(r.Body).Decode(&updatedInfo)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	if err = a.service.UpdateAccount(ctx, updatedInfo.FullName, updatedInfo.ShortName); err != nil {
		a.log.Error("failed to write json error response", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// Get gets authorized user and take it's params.
func (a *Auth) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var user struct {
		ID        uuid.UUID `json:"id"`
		FullName  string    `json:"fullName"`
		ShortName string    `json:"shortName"`
		Email     string    `json:"email"`
		PartnerID uuid.UUID `json:"partnerId"`
	}

	auth, err := console.GetAuth(ctx)
	if err != nil {
		a.serveJSONError(w, http.StatusUnauthorized, err)
		return
	}

	user.ShortName = auth.User.ShortName
	user.FullName = auth.User.FullName
	user.Email = auth.User.Email
	user.ID = auth.User.ID
	user.PartnerID = auth.User.PartnerID

	err = json.NewEncoder(w).Encode(&user)
	if err != nil {
		a.log.Error("could not encode user info", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// Delete - authorizes user and deletes account by password.
func (a *Auth) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var request struct {
		Password string `json:"password"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	err = a.service.DeleteAccount(ctx, request.Password)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			a.serveJSONError(w, http.StatusUnauthorized, err)
			return
		}

		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}
}

// ChangePassword auth user, changes users password for a new one.
func (a *Auth) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var passwordChange struct {
		CurrentPassword string `json:"password"`
		NewPassword     string `json:"newPassword"`
	}

	err = json.NewDecoder(r.Body).Decode(&passwordChange)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	err = a.service.ChangePassword(ctx, passwordChange.CurrentPassword, passwordChange.NewPassword)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			a.serveJSONError(w, http.StatusUnauthorized, err)
			return
		}

		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}
}

// ForgotPassword creates password-reset token and sends email to user.
func (a *Auth) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	params := mux.Vars(r)
	email, ok := params["email"]
	if !ok {
		err = errs.New("email expected")
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	user, err := a.service.GetUserByEmail(ctx, email)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}

	recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, user.ID)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}

	passwordRecoveryLink := a.ExternalAddress + consoleql.CancelPasswordRecoveryPath + recoveryToken
	cancelPasswordRecoveryLink := a.ExternalAddress + consoleql.CancelPasswordRecoveryPath + recoveryToken
	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	contactInfoURL := a.ContactInfoURL
	letUsKnowURL := a.LetUsKnowURL
	termsAndConditionsURL := a.TermsAndConditionsURL

	a.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.ForgotPasswordEmail{
			Origin:                     a.ExternalAddress,
			UserName:                   userName,
			ResetLink:                  passwordRecoveryLink,
			CancelPasswordRecoveryLink: cancelPasswordRecoveryLink,
			LetUsKnowURL:               letUsKnowURL,
			ContactInfoURL:             contactInfoURL,
			TermsAndConditionsURL:      termsAndConditionsURL,
		},
	)
}

// ResendEmail generates activation token by userID and sends email account activation email to user.
func (a *Auth) ResendEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	params := mux.Vars(r)
	val, ok := params["id"]
	if !ok {
		a.serveJSONError(w, http.StatusBadRequest, errs.New("id expected"))
		return
	}

	userID, err := uuid.Parse(val)
	if err != nil {
		a.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	user, err := a.service.GetUser(ctx, *userID)
	if err != nil {
		a.serveJSONError(w, http.StatusNotFound, err)
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

	contactInfoURL := a.ContactInfoURL
	termsAndConditionsURL := a.TermsAndConditionsURL

	a.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.AccountActivationEmail{
			Origin:                a.ExternalAddress,
			ActivationLink:        link,
			TermsAndConditionsURL: termsAndConditionsURL,
			ContactInfoURL:        contactInfoURL,
		},
	)
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
		a.log.Error("failed to write json error response", zap.Error(ErrAuthAPI.Wrap(err)))
	}
}
