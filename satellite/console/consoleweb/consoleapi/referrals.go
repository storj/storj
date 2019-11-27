// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/post"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/referrals"
)

// ErrReferralsAPI - console referrals api error type.
var ErrReferralsAPI = errs.Class("console referrals api error")

// Referrals is an api controller that exposes all referrals functionality.
type Referrals struct {
	log             *zap.Logger
	service         *referrals.Service
	consoleService  *console.Service
	mailService     *mailservice.Service
	ExternalAddress string
}

// NewReferrals is a constructor for api referrals controller.
func NewReferrals(log *zap.Logger, service *referrals.Service, consoleService *console.Service, mailService *mailservice.Service, externalAddress string) *Referrals {
	return &Referrals{
		log:             log,
		service:         service,
		consoleService:  consoleService,
		mailService:     mailService,
		ExternalAddress: externalAddress,
	}
}

// GetTokens returns referral tokens based on user ID.
func (controller *Referrals) GetTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	auth, err := console.GetAuth(ctx)
	if err != nil {
		controller.serveJSONError(w, err)
		return
	}

	tokens, err := controller.service.GetTokens(ctx, &auth.User.ID)
	if err != nil {
		controller.serveJSONError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(tokens)
	if err != nil {
		controller.log.Error("token handler could not encode token response", zap.Error(ErrReferralsAPI.Wrap(err)))
		return
	}
}

// Register creates new user, sends activation e-mail.
func (controller *Referrals) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var registerData struct {
		FullName      string `json:"fullName"`
		ShortName     string `json:"shortName"`
		Email         string `json:"email"`
		Password      string `json:"password"`
		ReferralToken string `json:"referralToken"`
	}

	err = json.NewDecoder(r.Body).Decode(&registerData)
	if err != nil {
		controller.serveJSONError(w, err)
		return
	}

	user, err := controller.service.CreateUser(ctx, registerData)
	if err != nil {
		controller.serveJSONError(w, err)
		return
	}
	token, err := controller.consoleService.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		controller.serveJSONError(w, err)
		return
	}

	link := controller.ExternalAddress + "activation/?token=" + token
	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	controller.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.AccountActivationEmail{
			ActivationLink: link,
			Origin:         controller.ExternalAddress,
		},
	)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(user.ID)
	if err != nil {
		controller.log.Error("registration handler could not encode userID", zap.Error(ErrReferralsAPI.Wrap(err)))
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (controller *Referrals) serveJSONError(w http.ResponseWriter, err error) {
	w.WriteHeader(controller.getStatusCode(err))

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		controller.log.Error("failed to write json error response", zap.Error(ErrReferralsAPI.Wrap(err)))
	}
}

// getStatusCode returns http.StatusCode depends on console error class.
func (controller *Referrals) getStatusCode(err error) int {
	switch {
	case console.ErrValidation.Has(err):
		return http.StatusBadRequest
	case console.ErrUnauthorized.Has(err):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}
