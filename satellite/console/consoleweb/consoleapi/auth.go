// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/http/requestid"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
)

var (
	// ErrAuthAPI - console auth api error type.
	ErrAuthAPI = errs.Class("consoleapi auth")

	// errNotImplemented is the error value used by handlers of this package to
	// response with status Not Implemented.
	errNotImplemented = errs.New("not implemented")
)

// Auth is an api controller that exposes all auth functionality.
type Auth struct {
	log                       *zap.Logger
	ExternalAddress           string
	LetUsKnowURL              string
	TermsAndConditionsURL     string
	ContactInfoURL            string
	GeneralRequestURL         string
	PasswordRecoveryURL       string
	CancelPasswordRecoveryURL string
	ActivateAccountURL        string
	ActivationCodeEnabled     bool
	SatelliteName             string
	service                   *console.Service
	accountFreezeService      *console.AccountFreezeService
	analytics                 *analytics.Service
	mailService               *mailservice.Service
	cookieAuth                *consolewebauth.CookieAuth
}

// NewAuth is a constructor for api auth controller.
func NewAuth(log *zap.Logger, service *console.Service, accountFreezeService *console.AccountFreezeService, mailService *mailservice.Service, cookieAuth *consolewebauth.CookieAuth, analytics *analytics.Service, satelliteName, externalAddress, letUsKnowURL, termsAndConditionsURL, contactInfoURL, generalRequestURL string, activationCodeEnabled bool) *Auth {
	return &Auth{
		log:                       log,
		ExternalAddress:           externalAddress,
		LetUsKnowURL:              letUsKnowURL,
		TermsAndConditionsURL:     termsAndConditionsURL,
		ContactInfoURL:            contactInfoURL,
		GeneralRequestURL:         generalRequestURL,
		SatelliteName:             satelliteName,
		PasswordRecoveryURL:       externalAddress + "password-recovery",
		CancelPasswordRecoveryURL: externalAddress + "cancel-password-recovery",
		ActivateAccountURL:        externalAddress + "activation",
		ActivationCodeEnabled:     activationCodeEnabled,
		service:                   service,
		accountFreezeService:      accountFreezeService,
		mailService:               mailService,
		cookieAuth:                cookieAuth,
		analytics:                 analytics,
	}
}

// Token authenticates user by credentials and returns auth token.
func (a *Auth) Token(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	tokenRequest := console.AuthUser{}
	err = json.NewDecoder(r.Body).Decode(&tokenRequest)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	tokenRequest.UserAgent = r.UserAgent()
	tokenRequest.IP, err = web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	tokenInfo, err := a.service.Token(ctx, tokenRequest)
	if err != nil {
		if console.ErrMFAMissing.Has(err) {
			web.ServeCustomJSONError(ctx, a.log, w, http.StatusOK, err, a.getUserErrorMessage(err))
		} else {
			a.log.Info("Error authenticating token request", zap.String("email", tokenRequest.Email), zap.Error(ErrAuthAPI.Wrap(err)))
			a.serveJSONError(ctx, w, err)
		}
		return
	}

	a.cookieAuth.SetTokenCookie(w, *tokenInfo)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		console.TokenInfo
		Token string `json:"token"`
	}{*tokenInfo, tokenInfo.Token.String()})
	if err != nil {
		a.log.Error("token handler could not encode token response", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// TokenByAPIKey authenticates user by API key and returns auth token.
func (a *Auth) TokenByAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	authToken := r.Header.Get("Authorization")
	if !(strings.HasPrefix(authToken, "Bearer ")) {
		a.log.Info("authorization key format is incorrect. Should be 'Bearer <key>'")
		a.serveJSONError(ctx, w, err)
		return
	}

	apiKey := strings.TrimPrefix(authToken, "Bearer ")

	userAgent := r.UserAgent()
	ip, err := web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	tokenInfo, err := a.service.TokenByAPIKey(ctx, userAgent, ip, apiKey)
	if err != nil {
		a.log.Info("Error authenticating token request", zap.Error(ErrAuthAPI.Wrap(err)))
		a.serveJSONError(ctx, w, err)
		return
	}

	a.cookieAuth.SetTokenCookie(w, *tokenInfo)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		console.TokenInfo
		Token string `json:"token"`
	}{*tokenInfo, tokenInfo.Token.String()})
	if err != nil {
		a.log.Error("token handler could not encode token response", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// getSessionID gets the session ID from the request.
func (a *Auth) getSessionID(r *http.Request) (id uuid.UUID, err error) {

	tokenInfo, err := a.cookieAuth.GetToken(r)
	if err != nil {
		return uuid.UUID{}, err
	}

	sessionID, err := uuid.FromBytes(tokenInfo.Token.Payload)
	if err != nil {
		return uuid.UUID{}, err
	}

	return sessionID, nil
}

// Logout removes auth cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	w.Header().Set("Content-Type", "application/json")

	sessionID, err := a.getSessionID(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.DeleteSession(ctx, sessionID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	a.cookieAuth.RemoveTokenCookie(w)
}

// Register creates new user, sends activation e-mail.
// If a user with the given e-mail address already exists, a password reset e-mail is sent instead.
func (a *Auth) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var registerData struct {
		FullName         string `json:"fullName"`
		ShortName        string `json:"shortName"`
		Email            string `json:"email"`
		Partner          string `json:"partner"`
		UserAgent        []byte `json:"userAgent"`
		Password         string `json:"password"`
		SecretInput      string `json:"secret"`
		ReferrerUserID   string `json:"referrerUserId"`
		IsProfessional   bool   `json:"isProfessional"`
		Position         string `json:"position"`
		CompanyName      string `json:"companyName"`
		StorageNeeds     string `json:"storageNeeds"`
		EmployeeCount    string `json:"employeeCount"`
		HaveSalesContact bool   `json:"haveSalesContact"`
		CaptchaResponse  string `json:"captchaResponse"`
		SignupPromoCode  string `json:"signupPromoCode"`
		IsMinimal        bool   `json:"isMinimal"`
	}

	err = json.NewDecoder(r.Body).Decode(&registerData)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	// trim leading and trailing spaces of email address.
	registerData.Email = strings.TrimSpace(registerData.Email)

	isValidEmail := utils.ValidateEmail(registerData.Email)
	if !isValidEmail {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("Invalid email.")))
		return
	}

	if len([]rune(registerData.Partner)) > 100 {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("Partner must be less than or equal to 100 characters")))
		return
	}

	if len([]rune(registerData.SignupPromoCode)) > 100 {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("Promo code must be less than or equal to 100 characters")))
		return
	}

	verified, unverified, err := a.service.GetUserByEmailWithUnverified(ctx, registerData.Email)
	if err != nil && !console.ErrEmailNotFound.Has(err) {
		a.serveJSONError(ctx, w, err)
		return
	}

	if verified != nil {
		satelliteAddress := a.ExternalAddress
		if !strings.HasSuffix(satelliteAddress, "/") {
			satelliteAddress += "/"
		}
		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: verified.Email}},
			&console.AccountAlreadyExistsEmail{
				Origin:            satelliteAddress,
				SatelliteName:     a.SatelliteName,
				SignInLink:        satelliteAddress + "login",
				ResetPasswordLink: satelliteAddress + "forgot-password",
				CreateAccountLink: satelliteAddress + "signup",
			},
		)
		return
	}

	var user *console.User
	if len(unverified) > 0 {
		user = &unverified[0]
	} else {
		secret, err := console.RegistrationSecretFromBase64(registerData.SecretInput)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		if registerData.Partner != "" {
			registerData.UserAgent = []byte(registerData.Partner)
		}

		ip, err := web.GetRequestIP(r)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		var code string
		var requestID string
		if a.ActivationCodeEnabled {
			randNum, err := rand.Int(rand.Reader, big.NewInt(900000))
			if err != nil {
				a.serveJSONError(ctx, w, console.Error.Wrap(err))
				return
			}
			randNum = randNum.Add(randNum, big.NewInt(100000))
			code = randNum.String()

			requestID = requestid.FromContext(ctx)
		}

		user, err = a.service.CreateUser(ctx,
			console.CreateUser{
				FullName:         registerData.FullName,
				ShortName:        registerData.ShortName,
				Email:            registerData.Email,
				UserAgent:        registerData.UserAgent,
				Password:         registerData.Password,
				IsProfessional:   registerData.IsProfessional,
				Position:         registerData.Position,
				CompanyName:      registerData.CompanyName,
				EmployeeCount:    registerData.EmployeeCount,
				HaveSalesContact: registerData.HaveSalesContact,
				CaptchaResponse:  registerData.CaptchaResponse,
				IP:               ip,
				SignupPromoCode:  registerData.SignupPromoCode,
				ActivationCode:   code,
				SignupId:         requestID,
				// the minimal signup from the v2 app doesn't require name.
				AllowNoName: registerData.IsMinimal,
			},
			secret,
		)
		if err != nil {
			if !console.ErrEmailUsed.Has(err) {
				a.serveJSONError(ctx, w, err)
			}
			return
		}

		invites, err := a.service.GetInvitesByEmail(ctx, registerData.Email)
		if err != nil {
			a.log.Error("Could not get invitations", zap.String("email", registerData.Email), zap.Error(err))
		} else if len(invites) > 0 {
			var firstInvite console.ProjectInvitation
			for _, inv := range invites {
				if inv.InviterID != nil && (firstInvite.CreatedAt.IsZero() || inv.CreatedAt.Before(firstInvite.CreatedAt)) {
					firstInvite = inv
				}
			}
			if firstInvite.InviterID != nil {
				inviter, err := a.service.GetUser(ctx, *firstInvite.InviterID)
				if err != nil {
					a.log.Error("Error getting inviter info", zap.String("ID", firstInvite.InviterID.String()), zap.Error(err))
				} else {
					a.analytics.TrackInviteLinkSignup(inviter.Email, registerData.Email)
				}
			}
		}

		// see if referrer was provided in URL query, otherwise use the Referer header in the request.
		referrer := r.URL.Query().Get("referrer")
		if referrer == "" {
			referrer = r.Referer()
		}
		hubspotUTK := ""
		hubspotCookie, err := r.Cookie("hubspotutk")
		if err == nil {
			hubspotUTK = hubspotCookie.Value
		}

		trackCreateUserFields := analytics.TrackCreateUserFields{
			ID:            user.ID,
			AnonymousID:   loadSession(r),
			FullName:      user.FullName,
			Email:         user.Email,
			Type:          analytics.Personal,
			OriginHeader:  r.Header.Get("Origin"),
			Referrer:      referrer,
			HubspotUTK:    hubspotUTK,
			UserAgent:     string(user.UserAgent),
			SignupCaptcha: user.SignupCaptcha,
		}
		if user.IsProfessional {
			trackCreateUserFields.Type = analytics.Professional
			trackCreateUserFields.EmployeeCount = user.EmployeeCount
			trackCreateUserFields.CompanyName = user.CompanyName
			trackCreateUserFields.StorageNeeds = registerData.StorageNeeds
			trackCreateUserFields.JobTitle = user.Position
			trackCreateUserFields.HaveSalesContact = user.HaveSalesContact
		}
		a.analytics.TrackCreateUser(trackCreateUserFields)
	}

	if a.ActivationCodeEnabled {
		*user, err = a.service.SetActivationCodeAndSignupID(ctx, *user)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email}},
			&console.AccountActivationCodeEmail{
				ActivationCode: user.ActivationCode,
			},
		)

		return
	}
	token, err := a.service.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	link := a.ActivateAccountURL + "?token=" + token

	a.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email}},
		&console.AccountActivationEmail{
			ActivationLink: link,
			Origin:         a.ExternalAddress,
		},
	)
}

// ActivateAccount verifies a signup activation code.
func (a *Auth) ActivateAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var activateData struct {
		Email    string `json:"email"`
		Code     string `json:"code"`
		SignupId string `json:"signupId"`
	}
	err = json.NewDecoder(r.Body).Decode(&activateData)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	verified, unverified, err := a.service.GetUserByEmailWithUnverified(ctx, activateData.Email)
	if err != nil && !console.ErrEmailNotFound.Has(err) {
		a.serveJSONError(ctx, w, err)
		return
	}

	if verified != nil {
		satelliteAddress := a.ExternalAddress
		if !strings.HasSuffix(satelliteAddress, "/") {
			satelliteAddress += "/"
		}
		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: verified.Email}},
			&console.AccountAlreadyExistsEmail{
				Origin:            satelliteAddress,
				SatelliteName:     a.SatelliteName,
				SignInLink:        satelliteAddress + "login",
				ResetPasswordLink: satelliteAddress + "forgot-password",
				CreateAccountLink: satelliteAddress + "signup",
			},
		)
		// return error since verified user already exists.
		a.serveJSONError(ctx, w, console.ErrUnauthorized.New("user already verified"))
		return
	}

	var user *console.User
	if len(unverified) == 0 {
		a.serveJSONError(ctx, w, console.ErrEmailNotFound.New("no unverified user found"))
		return
	}
	user = &unverified[0]

	if user.ActivationCode != activateData.Code || user.SignupId != activateData.SignupId {
		a.serveJSONError(ctx, w, console.ErrActivationCode.New("invalid activation code"))
		return
	}

	err = a.service.SetAccountActive(ctx, user)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	ip, err := web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	tokenInfo, err := a.service.GenerateSessionToken(ctx, user.ID, user.Email, ip, r.UserAgent(), nil)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	a.cookieAuth.SetTokenCookie(w, *tokenInfo)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(struct {
		console.TokenInfo
		Token string `json:"token"`
	}{*tokenInfo, tokenInfo.Token.String()})
	if err != nil {
		a.log.Error("could not encode token response", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// loadSession looks for a cookie for the session id.
// this cookie is set from the reverse proxy if the user opts into cookies from Storj.
func loadSession(req *http.Request) string {
	sessionCookie, err := req.Cookie("webtraf-sid")
	if err != nil {
		return ""
	}
	return sessionCookie.Value
}

// GetFreezeStatus checks to see if an account is frozen or warned.
func (a *Auth) GetFreezeStatus(w http.ResponseWriter, r *http.Request) {
	type FrozenResult struct {
		Frozen          bool `json:"frozen"`
		Warned          bool `json:"warned"`
		ViolationFrozen bool `json:"violationFrozen"`
	}

	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	userID, err := a.service.GetUserID(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	freezes, err := a.accountFreezeService.GetAll(ctx, userID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(FrozenResult{
		Frozen:          freezes.BillingFreeze != nil,
		Warned:          freezes.BillingWarning != nil,
		ViolationFrozen: freezes.ViolationFreeze != nil,
	})
	if err != nil {
		a.log.Error("could not encode account status", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// UpdateAccount updates user's full name and short name.
func (a *Auth) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var updatedInfo struct {
		FullName  string `json:"fullName"`
		ShortName string `json:"shortName"`
	}

	err = json.NewDecoder(r.Body).Decode(&updatedInfo)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	if err = a.service.UpdateAccount(ctx, updatedInfo.FullName, updatedInfo.ShortName); err != nil {
		a.serveJSONError(ctx, w, err)
	}
}

// SetupAccount updates user's full name and short name.
func (a *Auth) SetupAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var updatedInfo console.SetUpAccountRequest

	err = json.NewDecoder(r.Body).Decode(&updatedInfo)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	if err = a.service.SetupAccount(ctx, updatedInfo); err != nil {
		a.serveJSONError(ctx, w, err)
	}
}

// GetAccount gets authorized user and take it's params.
func (a *Auth) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var user struct {
		ID                    uuid.UUID `json:"id"`
		FullName              string    `json:"fullName"`
		ShortName             string    `json:"shortName"`
		Email                 string    `json:"email"`
		Partner               string    `json:"partner"`
		ProjectLimit          int       `json:"projectLimit"`
		ProjectStorageLimit   int64     `json:"projectStorageLimit"`
		ProjectBandwidthLimit int64     `json:"projectBandwidthLimit"`
		ProjectSegmentLimit   int64     `json:"projectSegmentLimit"`
		IsProfessional        bool      `json:"isProfessional"`
		Position              string    `json:"position"`
		CompanyName           string    `json:"companyName"`
		EmployeeCount         string    `json:"employeeCount"`
		HaveSalesContact      bool      `json:"haveSalesContact"`
		PaidTier              bool      `json:"paidTier"`
		MFAEnabled            bool      `json:"isMFAEnabled"`
		MFARecoveryCodeCount  int       `json:"mfaRecoveryCodeCount"`
		CreatedAt             time.Time `json:"createdAt"`
		PendingVerification   bool      `json:"pendingVerification"`
	}

	consoleUser, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	user.ShortName = consoleUser.ShortName
	user.FullName = consoleUser.FullName
	user.Email = consoleUser.Email
	user.ID = consoleUser.ID
	if consoleUser.UserAgent != nil {
		user.Partner = string(consoleUser.UserAgent)
	}
	user.ProjectLimit = consoleUser.ProjectLimit
	user.ProjectStorageLimit = consoleUser.ProjectStorageLimit
	user.ProjectBandwidthLimit = consoleUser.ProjectBandwidthLimit
	user.ProjectSegmentLimit = consoleUser.ProjectSegmentLimit
	user.IsProfessional = consoleUser.IsProfessional
	user.CompanyName = consoleUser.CompanyName
	user.Position = consoleUser.Position
	user.EmployeeCount = consoleUser.EmployeeCount
	user.HaveSalesContact = consoleUser.HaveSalesContact
	user.PaidTier = consoleUser.PaidTier
	user.MFAEnabled = consoleUser.MFAEnabled
	user.MFARecoveryCodeCount = len(consoleUser.MFARecoveryCodes)
	user.CreatedAt = consoleUser.CreatedAt
	user.PendingVerification = consoleUser.Status == console.PendingBotVerification

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&user)
	if err != nil {
		a.log.Error("could not encode user info", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// DeleteAccount authorizes user and deletes account by password.
func (a *Auth) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(&errNotImplemented)

	// We do not want to allow account deletion via API currently.
	a.serveJSONError(ctx, w, errNotImplemented)
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
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.ChangePassword(ctx, passwordChange.CurrentPassword, passwordChange.NewPassword)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
}

// ForgotPassword creates password-reset token and sends email to user.
func (a *Auth) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var forgotPassword struct {
		Email           string `json:"email"`
		CaptchaResponse string `json:"captchaResponse"`
	}

	err = json.NewDecoder(r.Body).Decode(&forgotPassword)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	ip, err := web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	valid, err := a.service.VerifyForgotPasswordCaptcha(ctx, forgotPassword.CaptchaResponse, ip)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
	if !valid {
		a.serveJSONError(ctx, w, console.ErrCaptcha.New("captcha validation unsuccessful"))
		return
	}

	user, _, err := a.service.GetUserByEmailWithUnverified(ctx, forgotPassword.Email)
	if err != nil || user == nil {
		satelliteAddress := a.ExternalAddress

		if !strings.HasSuffix(satelliteAddress, "/") {
			satelliteAddress += "/"
		}
		resetPasswordLink := satelliteAddress + "forgot-password"
		doubleCheckLink := satelliteAddress + "login"
		createAccountLink := satelliteAddress + "signup"

		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: forgotPassword.Email, Name: ""}},
			&console.UnknownResetPasswordEmail{
				Satellite:           a.SatelliteName,
				Email:               forgotPassword.Email,
				DoubleCheckLink:     doubleCheckLink,
				ResetPasswordLink:   resetPasswordLink,
				CreateAnAccountLink: createAccountLink,
				SupportTeamLink:     a.GeneralRequestURL,
			},
		)
		return
	}

	recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, user.ID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	passwordRecoveryLink := a.PasswordRecoveryURL + "?token=" + recoveryToken
	cancelPasswordRecoveryLink := a.CancelPasswordRecoveryURL + "?token=" + recoveryToken
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
		&console.ForgotPasswordEmail{
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

// ResendEmail generates activation token by e-mail address and sends email account activation email to user.
// If the account is already activated, a password reset e-mail is sent instead.
func (a *Auth) ResendEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	params := mux.Vars(r)
	email, ok := params["email"]
	if !ok {
		return
	}

	verified, unverified, err := a.service.GetUserByEmailWithUnverified(ctx, email)
	if err != nil {
		return
	}

	if verified != nil {
		recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, verified.ID)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		userName := verified.ShortName
		if verified.ShortName == "" {
			userName = verified.FullName
		}

		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: verified.Email, Name: userName}},
			&console.ForgotPasswordEmail{
				Origin:                     a.ExternalAddress,
				UserName:                   userName,
				ResetLink:                  a.PasswordRecoveryURL + "?token=" + recoveryToken,
				CancelPasswordRecoveryLink: a.CancelPasswordRecoveryURL + "?token=" + recoveryToken,
				LetUsKnowURL:               a.LetUsKnowURL,
				ContactInfoURL:             a.ContactInfoURL,
				TermsAndConditionsURL:      a.TermsAndConditionsURL,
			},
		)
		return
	}

	user := unverified[0]

	if a.ActivationCodeEnabled {
		user, err = a.service.SetActivationCodeAndSignupID(ctx, user)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email}},
			&console.AccountActivationCodeEmail{
				ActivationCode: user.ActivationCode,
			},
		)

		return
	}

	token, err := a.service.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	link := a.ActivateAccountURL + "?token=" + token
	contactInfoURL := a.ContactInfoURL
	termsAndConditionsURL := a.TermsAndConditionsURL

	a.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email}},
		&console.AccountActivationEmail{
			Origin:                a.ExternalAddress,
			ActivationLink:        link,
			TermsAndConditionsURL: termsAndConditionsURL,
			ContactInfoURL:        contactInfoURL,
		},
	)
}

// EnableUserMFA enables multi-factor authentication for the user.
func (a *Auth) EnableUserMFA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data struct {
		Passcode string `json:"passcode"`
	}
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.EnableUserMFA(ctx, data.Passcode, time.Now())
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	sessionID, err := a.getSessionID(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	consoleUser, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.DeleteAllSessionsByUserIDExcept(ctx, consoleUser.ID, sessionID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
}

// DisableUserMFA disables multi-factor authentication for the user.
func (a *Auth) DisableUserMFA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data struct {
		Passcode     string `json:"passcode"`
		RecoveryCode string `json:"recoveryCode"`
	}
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.DisableUserMFA(ctx, data.Passcode, time.Now(), data.RecoveryCode)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	sessionID, err := a.getSessionID(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	consoleUser, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.DeleteAllSessionsByUserIDExcept(ctx, consoleUser.ID, sessionID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
}

// GenerateMFASecretKey creates a new TOTP secret key for the user.
func (a *Auth) GenerateMFASecretKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	key, err := a.service.ResetMFASecretKey(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(key)
	if err != nil {
		a.log.Error("could not encode MFA secret key", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// GenerateMFARecoveryCodes creates a new set of MFA recovery codes for the user.
func (a *Auth) GenerateMFARecoveryCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	codes, err := a.service.ResetMFARecoveryCodes(ctx, false, "", "")
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(codes)
	if err != nil {
		a.log.Error("could not encode MFA recovery codes", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// RegenerateMFARecoveryCodes requires MFA code to create a new set of MFA recovery codes for the user.
func (a *Auth) RegenerateMFARecoveryCodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data struct {
		Passcode     string `json:"passcode"`
		RecoveryCode string `json:"recoveryCode"`
	}
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	codes, err := a.service.ResetMFARecoveryCodes(ctx, true, data.Passcode, data.RecoveryCode)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(codes)
	if err != nil {
		a.log.Error("could not encode MFA recovery codes", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// ResetPassword resets user's password using recovery token.
func (a *Auth) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var resetPassword struct {
		RecoveryToken   string `json:"token"`
		NewPassword     string `json:"password"`
		MFAPasscode     string `json:"mfaPasscode"`
		MFARecoveryCode string `json:"mfaRecoveryCode"`
	}

	err = json.NewDecoder(r.Body).Decode(&resetPassword)
	if err != nil {
		a.serveJSONError(ctx, w, err)
	}

	err = a.service.ResetPassword(ctx, resetPassword.RecoveryToken, resetPassword.NewPassword, resetPassword.MFAPasscode, resetPassword.MFARecoveryCode, time.Now())

	if console.ErrMFAMissing.Has(err) || console.ErrMFAPasscode.Has(err) || console.ErrMFARecoveryCode.Has(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(a.getStatusCode(err))

		err = json.NewEncoder(w).Encode(map[string]string{
			"error": a.getUserErrorMessage(err),
			"code":  "mfa_required",
		})

		if err != nil {
			a.log.Error("failed to write json response", zap.Error(ErrUtils.Wrap(err)))
		}

		return
	}

	if console.ErrTokenExpiration.Has(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(a.getStatusCode(err))

		err = json.NewEncoder(w).Encode(map[string]string{
			"error": a.getUserErrorMessage(err),
			"code":  "token_expired",
		})

		if err != nil {
			a.log.Error("password-reset-token expired: failed to write json response", zap.Error(ErrUtils.Wrap(err)))
		}

		return
	}

	if err != nil {
		a.serveJSONError(ctx, w, err)
	} else {
		a.cookieAuth.RemoveTokenCookie(w)
	}
}

// RefreshSession refreshes the user's session.
func (a *Auth) RefreshSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	tokenInfo, err := a.cookieAuth.GetToken(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	id, err := uuid.FromBytes(tokenInfo.Token.Payload)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	tokenInfo.ExpiresAt, err = a.service.RefreshSession(ctx, id)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	a.cookieAuth.SetTokenCookie(w, tokenInfo)

	err = json.NewEncoder(w).Encode(tokenInfo.ExpiresAt)
	if err != nil {
		a.log.Error("could not encode refreshed session expiration date", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// GetUserSettings gets a user's settings.
func (a *Auth) GetUserSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	settings, err := a.service.GetUserSettings(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = json.NewEncoder(w).Encode(settings)
	if err != nil {
		a.log.Error("could not encode settings", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// SetOnboardingStatus updates a user's onboarding status.
func (a *Auth) SetOnboardingStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var updateInfo struct {
		OnboardingStart *bool   `json:"onboardingStart"`
		OnboardingEnd   *bool   `json:"onboardingEnd"`
		OnboardingStep  *string `json:"onboardingStep"`
	}

	err = json.NewDecoder(r.Body).Decode(&updateInfo)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	_, err = a.service.SetUserSettings(ctx, console.UpsertUserSettingsRequest{
		OnboardingStart: updateInfo.OnboardingStart,
		OnboardingEnd:   updateInfo.OnboardingEnd,
		OnboardingStep:  updateInfo.OnboardingStep,
	})
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
}

// SetUserSettings updates a user's settings.
func (a *Auth) SetUserSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var updateInfo struct {
		OnboardingStart  *bool   `json:"onboardingStart"`
		OnboardingEnd    *bool   `json:"onboardingEnd"`
		PassphrasePrompt *bool   `json:"passphrasePrompt"`
		OnboardingStep   *string `json:"onboardingStep"`
		SessionDuration  *int64  `json:"sessionDuration"`
	}

	err = json.NewDecoder(r.Body).Decode(&updateInfo)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	var newDuration **time.Duration
	if updateInfo.SessionDuration != nil {
		newDuration = new(*time.Duration)
		if *updateInfo.SessionDuration != 0 {
			duration := time.Duration(*updateInfo.SessionDuration)
			*newDuration = &duration
		}
	}

	settings, err := a.service.SetUserSettings(ctx, console.UpsertUserSettingsRequest{
		OnboardingStart:  updateInfo.OnboardingStart,
		OnboardingEnd:    updateInfo.OnboardingEnd,
		OnboardingStep:   updateInfo.OnboardingStep,
		PassphrasePrompt: updateInfo.PassphrasePrompt,
		SessionDuration:  newDuration,
	})
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = json.NewEncoder(w).Encode(settings)
	if err != nil {
		a.log.Error("could not encode settings", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// RequestLimitIncrease handles requesting increase for project limit.
func (a *Auth) RequestLimitIncrease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		a.serveJSONError(ctx, w, err)
	}

	err = a.service.RequestProjectLimitIncrease(ctx, string(b))
	if err != nil {
		a.serveJSONError(ctx, w, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (a *Auth) serveJSONError(ctx context.Context, w http.ResponseWriter, err error) {
	status := a.getStatusCode(err)
	web.ServeCustomJSONError(ctx, a.log, w, status, err, a.getUserErrorMessage(err))
}

// getStatusCode returns http.StatusCode depends on console error class.
func (a *Auth) getStatusCode(err error) int {
	var maxBytesError *http.MaxBytesError

	switch {
	case console.ErrValidation.Has(err), console.ErrCaptcha.Has(err), console.ErrMFAMissing.Has(err), console.ErrMFAPasscode.Has(err), console.ErrMFARecoveryCode.Has(err), console.ErrChangePassword.Has(err), console.ErrInvalidProjectLimit.Has(err):
		return http.StatusBadRequest
	case console.ErrUnauthorized.Has(err), console.ErrTokenExpiration.Has(err), console.ErrRecoveryToken.Has(err), console.ErrLoginCredentials.Has(err), console.ErrActivationCode.Has(err):
		return http.StatusUnauthorized
	case console.ErrEmailUsed.Has(err), console.ErrMFAConflict.Has(err):
		return http.StatusConflict
	case errors.Is(err, errNotImplemented):
		return http.StatusNotImplemented
	case console.ErrNotPaidTier.Has(err):
		return http.StatusPaymentRequired
	case errors.As(err, &maxBytesError):
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

// getUserErrorMessage returns a user-friendly representation of the error.
func (a *Auth) getUserErrorMessage(err error) string {
	var maxBytesError *http.MaxBytesError

	switch {
	case console.ErrCaptcha.Has(err):
		return "Validation of captcha was unsuccessful"
	case console.ErrRegToken.Has(err):
		return "We are unable to create your account. This is an invite-only alpha, please join our waitlist to receive an invitation"
	case console.ErrEmailUsed.Has(err):
		return "This email is already in use; try another"
	case console.ErrRecoveryToken.Has(err):
		if console.ErrTokenExpiration.Has(err) {
			return "The recovery token has expired"
		}
		return "The recovery token is invalid"
	case console.ErrMFAMissing.Has(err):
		return "A MFA passcode or recovery code is required"
	case console.ErrMFAConflict.Has(err):
		return "Expected either passcode or recovery code, but got both"
	case console.ErrMFAPasscode.Has(err):
		return "The MFA passcode is not valid or has expired"
	case console.ErrMFARecoveryCode.Has(err):
		return "The MFA recovery code is not valid or has been previously used"
	case console.ErrLoginCredentials.Has(err):
		return "Your login credentials are incorrect, please try again"
	case console.ErrValidation.Has(err), console.ErrChangePassword.Has(err), console.ErrInvalidProjectLimit.Has(err), console.ErrNotPaidTier.Has(err):
		return err.Error()
	case errors.Is(err, errNotImplemented):
		return "The server is incapable of fulfilling the request"
	case errors.As(err, &maxBytesError):
		return "Request body is too large"
	case console.ErrActivationCode.Has(err):
		return "The activation code is invalid"
	default:
		return "There was an error processing your request"
	}
}
