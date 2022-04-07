// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/rewards"
)

var (
	// ErrAuthAPI - console auth api error type.
	ErrAuthAPI = errs.Class("consoleapi auth")

	// errNotImplemented is the error value used by handlers of this package to
	// response with status Not Implemented.
	errNotImplemented = errs.New("not implemented")

	// supportedCORSOrigins allows us to support visitors who sign up from the website.
	supportedCORSOrigins = map[string]bool{
		"https://storj.io":     true,
		"https://www.storj.io": true,
	}
)

// Auth is an api controller that exposes all auth functionality.
type Auth struct {
	log                       *zap.Logger
	ExternalAddress           string
	LetUsKnowURL              string
	TermsAndConditionsURL     string
	ContactInfoURL            string
	PasswordRecoveryURL       string
	CancelPasswordRecoveryURL string
	ActivateAccountURL        string
	service                   *console.Service
	analytics                 *analytics.Service
	mailService               *mailservice.Service
	cookieAuth                *consolewebauth.CookieAuth
	partners                  *rewards.PartnersService
}

// NewAuth is a constructor for api auth controller.
func NewAuth(log *zap.Logger, service *console.Service, mailService *mailservice.Service, cookieAuth *consolewebauth.CookieAuth, partners *rewards.PartnersService, analytics *analytics.Service, externalAddress string, letUsKnowURL string, termsAndConditionsURL string, contactInfoURL string) *Auth {
	return &Auth{
		log:                       log,
		ExternalAddress:           externalAddress,
		LetUsKnowURL:              letUsKnowURL,
		TermsAndConditionsURL:     termsAndConditionsURL,
		ContactInfoURL:            contactInfoURL,
		PasswordRecoveryURL:       externalAddress + "password-recovery/",
		CancelPasswordRecoveryURL: externalAddress + "cancel-password-recovery/",
		ActivateAccountURL:        externalAddress + "activation/",
		service:                   service,
		mailService:               mailService,
		cookieAuth:                cookieAuth,
		partners:                  partners,
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
		a.serveJSONError(w, err)
		return
	}

	token, err := a.service.Token(ctx, tokenRequest)
	if err != nil {
		if console.ErrMFAMissing.Has(err) {
			serveCustomJSONError(a.log, w, 200, err, a.getUserErrorMessage(err))
		} else {
			a.log.Info("Error authenticating token request", zap.String("email", tokenRequest.Email), zap.Error(ErrAuthAPI.Wrap(err)))
			a.serveJSONError(w, err)
		}
		return
	}

	a.cookieAuth.SetTokenCookie(w, token)

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(token)
	if err != nil {
		a.log.Error("token handler could not encode token response", zap.Error(ErrAuthAPI.Wrap(err)))
		return
	}
}

// Logout removes auth cookie.
func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	a.cookieAuth.RemoveTokenCookie(w)

	w.Header().Set("Content-Type", "application/json")
}

// replaceURLCharacters replaces slash, colon, and dot characters in a string with a hyphen.
func replaceURLCharacters(s string) string {
	re := regexp.MustCompile(`[\/:\.]`)
	return re.ReplaceAllString(s, "-")
}

// Register creates new user, sends activation e-mail.
// If a user with the given e-mail address already exists, a password reset e-mail is sent instead.
func (a *Auth) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	origin := r.Header.Get("Origin")
	if supportedCORSOrigins[origin] {
		// we should send the exact origin back, rather than a wildcard
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
	}

	// OPTIONS is a pre-flight check for cross-origin (CORS) permissions
	if r.Method == "OPTIONS" {
		return
	}

	var userID uuid.UUID
	defer func() {
		if err == nil {
			w.Header().Set("Content-Type", "application/json")
			err = json.NewEncoder(w).Encode(userID)
			if err != nil {
				a.log.Error("registration handler could not encode userID", zap.Error(ErrAuthAPI.Wrap(err)))
			}
		}
	}()

	var registerData struct {
		FullName          string `json:"fullName"`
		ShortName         string `json:"shortName"`
		Email             string `json:"email"`
		Partner           string `json:"partner"`
		PartnerID         string `json:"partnerId"`
		UserAgent         []byte `json:"userAgent"`
		Password          string `json:"password"`
		SecretInput       string `json:"secret"`
		ReferrerUserID    string `json:"referrerUserId"`
		IsProfessional    bool   `json:"isProfessional"`
		Position          string `json:"position"`
		CompanyName       string `json:"companyName"`
		EmployeeCount     string `json:"employeeCount"`
		HaveSalesContact  bool   `json:"haveSalesContact"`
		RecaptchaResponse string `json:"recaptchaResponse"`
		SignupPromoCode   string `json:"signupPromoCode"`
	}

	err = json.NewDecoder(r.Body).Decode(&registerData)
	if err != nil {
		a.serveJSONError(w, err)
		return
	}

	// trim leading and trailing spaces of email address.
	registerData.Email = strings.TrimSpace(registerData.Email)

	isValidEmail := ValidateEmail(registerData.Email)
	if !isValidEmail {
		a.serveJSONError(w, console.ErrValidation.Wrap(errs.New("Invalid email.")))
		return
	}

	// remove special characters from submitted name so that malicious link cannot be injected into verification or password reset emails.
	registerData.FullName = replaceURLCharacters(registerData.FullName)
	registerData.ShortName = replaceURLCharacters(registerData.ShortName)

	verified, unverified, err := a.service.GetUserByEmailWithUnverified(ctx, registerData.Email)
	if err != nil && !console.ErrEmailNotFound.Has(err) {
		a.serveJSONError(w, err)
		return
	}

	if verified != nil {
		recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, verified.ID)
		if err != nil {
			a.serveJSONError(w, err)
			return
		}

		userName := verified.ShortName
		if verified.ShortName == "" {
			userName = verified.FullName
		}

		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: verified.Email, Name: userName}},
			&consoleql.ForgotPasswordEmail{
				Origin:                     a.ExternalAddress,
				UserName:                   userName,
				ResetLink:                  a.PasswordRecoveryURL + "?token=" + recoveryToken,
				CancelPasswordRecoveryLink: a.CancelPasswordRecoveryURL + "?token=" + recoveryToken,
				LetUsKnowURL:               a.LetUsKnowURL,
				ContactInfoURL:             a.ContactInfoURL,
				TermsAndConditionsURL:      a.TermsAndConditionsURL,
			},
		)
		userID = verified.ID
		return
	}

	var user *console.User
	if len(unverified) > 0 {
		user = &unverified[0]
	} else {
		secret, err := console.RegistrationSecretFromBase64(registerData.SecretInput)
		if err != nil {
			a.serveJSONError(w, err)
			return
		}

		if registerData.Partner != "" {
			registerData.UserAgent = []byte(registerData.Partner)
			info, err := a.partners.ByName(ctx, registerData.Partner)
			if err != nil {
				a.log.Warn("Invalid partner name", zap.String("Partner name", registerData.Partner), zap.String("User email", registerData.Email), zap.Error(err))
			} else {
				registerData.PartnerID = info.ID
			}
		}

		ip, err := web.GetRequestIP(r)
		if err != nil {
			a.serveJSONError(w, err)
			return
		}

		user, err = a.service.CreateUser(ctx,
			console.CreateUser{
				FullName:          registerData.FullName,
				ShortName:         registerData.ShortName,
				Email:             registerData.Email,
				PartnerID:         registerData.PartnerID,
				UserAgent:         registerData.UserAgent,
				Password:          registerData.Password,
				IsProfessional:    registerData.IsProfessional,
				Position:          registerData.Position,
				CompanyName:       registerData.CompanyName,
				EmployeeCount:     registerData.EmployeeCount,
				HaveSalesContact:  registerData.HaveSalesContact,
				RecaptchaResponse: registerData.RecaptchaResponse,
				IP:                ip,
				SignupPromoCode:   registerData.SignupPromoCode,
			},
			secret,
		)
		if err != nil {
			a.serveJSONError(w, err)
			return
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
			ID:           user.ID,
			AnonymousID:  loadSession(r),
			FullName:     user.FullName,
			Email:        user.Email,
			Type:         analytics.Personal,
			OriginHeader: origin,
			Referrer:     referrer,
			HubspotUTK:   hubspotUTK,
		}
		if user.IsProfessional {
			trackCreateUserFields.Type = analytics.Professional
			trackCreateUserFields.EmployeeCount = user.EmployeeCount
			trackCreateUserFields.CompanyName = user.CompanyName
			trackCreateUserFields.JobTitle = user.Position
			trackCreateUserFields.HaveSalesContact = user.HaveSalesContact
		}
		a.analytics.TrackCreateUser(trackCreateUserFields)
	}
	userID = user.ID

	token, err := a.service.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		a.serveJSONError(w, err)
		return
	}

	link := a.ActivateAccountURL + "?token=" + token
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
			UserName:       userName,
		},
	)

	if err = a.service.UpdateEmailVerificationReminder(ctx, time.Now().UTC()); err != nil {
		a.serveJSONError(w, err)
	}
}

// ValidateEmail validates email to have correct form and syntax.
func ValidateEmail(email string) bool {
	// This regular expression was built according to RFC 5322 and then extended to include international characters.
	re := regexp.MustCompile(`^(?:[a-z0-9\p{L}!#$%&'*+/=?^_{|}~\x60-]+(?:\.[a-z0-9\p{L}!#$%&'*+/=?^_{|}~\x60-]+)*|"(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21\x23-\x5b\x5d-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])*")@(?:(?:[a-z0-9\p{L}](?:[a-z0-9\p{L}-]*[a-z0-9\p{L}])?\.)+[a-z0-9\p{L}](?:[a-z\p{L}]*[a-z\p{L}])?|\[(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?|[a-z0-9\p{L}-]*[a-z0-9\p{L}]:(?:[\x01-\x08\x0b\x0c\x0e-\x1f\x21-\x5a\x53-\x7f]|\\[\x01-\x09\x0b\x0c\x0e-\x7f])+)\])$`)
	match := re.MatchString(email)

	return match
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
		a.serveJSONError(w, err)
		return
	}

	if err = a.service.UpdateAccount(ctx, updatedInfo.FullName, updatedInfo.ShortName); err != nil {
		a.serveJSONError(w, err)
	}
}

// GetAccount gets authorized user and take it's params.
func (a *Auth) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var user struct {
		ID                   uuid.UUID `json:"id"`
		FullName             string    `json:"fullName"`
		ShortName            string    `json:"shortName"`
		Email                string    `json:"email"`
		PartnerID            uuid.UUID `json:"partnerId"`
		UserAgent            []byte    `json:"userAgent"`
		ProjectLimit         int       `json:"projectLimit"`
		IsProfessional       bool      `json:"isProfessional"`
		Position             string    `json:"position"`
		CompanyName          string    `json:"companyName"`
		EmployeeCount        string    `json:"employeeCount"`
		HaveSalesContact     bool      `json:"haveSalesContact"`
		PaidTier             bool      `json:"paidTier"`
		MFAEnabled           bool      `json:"isMFAEnabled"`
		MFARecoveryCodeCount int       `json:"mfaRecoveryCodeCount"`
	}

	auth, err := console.GetAuth(ctx)
	if err != nil {
		a.serveJSONError(w, err)
		return
	}

	user.ShortName = auth.User.ShortName
	user.FullName = auth.User.FullName
	user.Email = auth.User.Email
	user.ID = auth.User.ID
	user.PartnerID = auth.User.PartnerID
	user.UserAgent = auth.User.UserAgent
	user.ProjectLimit = auth.User.ProjectLimit
	user.IsProfessional = auth.User.IsProfessional
	user.CompanyName = auth.User.CompanyName
	user.Position = auth.User.Position
	user.EmployeeCount = auth.User.EmployeeCount
	user.HaveSalesContact = auth.User.HaveSalesContact
	user.PaidTier = auth.User.PaidTier
	user.MFAEnabled = auth.User.MFAEnabled
	user.MFARecoveryCodeCount = len(auth.User.MFARecoveryCodes)

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
	a.serveJSONError(w, errNotImplemented)
}

// ChangeEmail auth user, changes users email for a new one.
func (a *Auth) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var emailChange struct {
		NewEmail string `json:"newEmail"`
	}

	err = json.NewDecoder(r.Body).Decode(&emailChange)
	if err != nil {
		a.serveJSONError(w, err)
		return
	}

	err = a.service.ChangeEmail(ctx, emailChange.NewEmail)
	if err != nil {
		a.serveJSONError(w, err)
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
		a.serveJSONError(w, err)
		return
	}

	err = a.service.ChangePassword(ctx, passwordChange.CurrentPassword, passwordChange.NewPassword)
	if err != nil {
		a.serveJSONError(w, err)
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
		a.serveJSONError(w, err)
		return
	}

	user, _, err := a.service.GetUserByEmailWithUnverified(ctx, email)
	if err != nil || user == nil {
		return
	}

	recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, user.ID)
	if err != nil {
		a.serveJSONError(w, err)
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
			a.serveJSONError(w, err)
			return
		}

		userName := verified.ShortName
		if verified.ShortName == "" {
			userName = verified.FullName
		}

		a.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: verified.Email, Name: userName}},
			&consoleql.ForgotPasswordEmail{
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

	token, err := a.service.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		a.serveJSONError(w, err)
		return
	}

	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	link := a.ActivateAccountURL + "?token=" + token
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
			UserName:              userName,
		},
	)

	if err = a.service.UpdateEmailVerificationReminder(ctx, time.Now().UTC()); err != nil {
		a.serveJSONError(w, err)
	}
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
		a.serveJSONError(w, err)
		return
	}

	err = a.service.EnableUserMFA(ctx, data.Passcode, time.Now())
	if err != nil {
		a.serveJSONError(w, err)
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
		a.serveJSONError(w, err)
		return
	}

	err = a.service.DisableUserMFA(ctx, data.Passcode, time.Now(), data.RecoveryCode)
	if err != nil {
		a.serveJSONError(w, err)
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
		a.serveJSONError(w, err)
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

	codes, err := a.service.ResetMFARecoveryCodes(ctx)
	if err != nil {
		a.serveJSONError(w, err)
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
		a.serveJSONError(w, err)
	}

	err = a.service.ResetPassword(ctx, resetPassword.RecoveryToken, resetPassword.NewPassword, resetPassword.MFAPasscode, resetPassword.MFARecoveryCode, time.Now())

	if console.ErrMFAMissing.Has(err) || console.ErrMFAPasscode.Has(err) || console.ErrMFARecoveryCode.Has(err) {
		w.WriteHeader(a.getStatusCode(err))
		w.Header().Set("Content-Type", "application/json")

		err = json.NewEncoder(w).Encode(map[string]string{
			"error": a.getUserErrorMessage(err),
			"code":  "mfa_required",
		})

		if err != nil {
			a.log.Error("failed to write json response", zap.Error(ErrUtils.Wrap(err)))
		}

		return
	}

	if err != nil {
		a.serveJSONError(w, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (a *Auth) serveJSONError(w http.ResponseWriter, err error) {
	status := a.getStatusCode(err)
	serveCustomJSONError(a.log, w, status, err, a.getUserErrorMessage(err))
}

// getStatusCode returns http.StatusCode depends on console error class.
func (a *Auth) getStatusCode(err error) int {
	switch {
	case console.ErrValidation.Has(err), console.ErrRecaptcha.Has(err), console.ErrMFAMissing.Has(err):
		return http.StatusBadRequest
	case console.ErrUnauthorized.Has(err), console.ErrRecoveryToken.Has(err), console.ErrLoginCredentials.Has(err):
		return http.StatusUnauthorized
	case console.ErrEmailUsed.Has(err), console.ErrMFAConflict.Has(err):
		return http.StatusConflict
	case errors.Is(err, errNotImplemented):
		return http.StatusNotImplemented
	case console.ErrMFAPasscode.Has(err), console.ErrMFARecoveryCode.Has(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// getUserErrorMessage returns a user-friendly representation of the error.
func (a *Auth) getUserErrorMessage(err error) string {
	switch {
	case console.ErrRecaptcha.Has(err):
		return "Validation of reCAPTCHA was unsuccessful"
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
	case errors.Is(err, errNotImplemented):
		return "The server is incapable of fulfilling the request"
	default:
		return "There was an error processing your request"
	}
}
