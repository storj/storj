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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"

	"storj.io/common/http/requestid"
	"storj.io/common/uuid"
	"storj.io/storj/private/post"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/csrf"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/tenancy"
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
	MemberAccountsEnabled     bool
	SatelliteName             string
	badPasswords              map[string]struct{}
	badPasswordsEncoded       string
	validAnnouncementNames    []string
	service                   *console.Service
	accountFreezeService      *console.AccountFreezeService
	analytics                 *analytics.Service
	mailService               *mailservice.Service
	ssoService                *sso.Service
	csrfService               *csrf.Service
	cookieAuth                *consolewebauth.CookieAuth
}

// NewAuth is a constructor for api auth controller.
func NewAuth(
	log *zap.Logger, service *console.Service, accountFreezeService *console.AccountFreezeService, mailService *mailservice.Service,
	cookieAuth *consolewebauth.CookieAuth, analytics *analytics.Service, ssoService *sso.Service, csrfService *csrf.Service,
	satelliteName, externalAddress, letUsKnowURL, termsAndConditionsURL, contactInfoURL, generalRequestURL string,
	activationCodeEnabled, memberAccountsEnabled bool, badPasswords map[string]struct{}, badPasswordsEncoded string, validAnnouncementNames []string,
) *Auth {
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
		MemberAccountsEnabled:     memberAccountsEnabled,
		service:                   service,
		accountFreezeService:      accountFreezeService,
		mailService:               mailService,
		cookieAuth:                cookieAuth,
		analytics:                 analytics,
		badPasswords:              badPasswords,
		badPasswordsEncoded:       badPasswordsEncoded,
		ssoService:                ssoService,
		csrfService:               csrfService,
		validAnnouncementNames:    validAnnouncementNames,
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

	if tokenRequest.Password == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("password is required"))
		return
	}

	tokenRequest.UserAgent = r.UserAgent()
	tokenRequest.IP, err = web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
	tokenRequest.AnonymousID = LoadAjsAnonymousID(r)

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

// AuthenticateSso logs in/signs up a user using already authenticated
// SSO provider.
func (a *Auth) AuthenticateSso(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	ssoFailedAddr := strings.TrimSuffix(a.ExternalAddress, "/") + "/login?sso_failed=true"

	provider := mux.Vars(r)["provider"]

	stateCookie, err := r.Cookie(a.cookieAuth.GetSSOStateCookieName())
	if err != nil {
		a.log.Error("Error verifying SSO auth", zap.Error(console.ErrValidation.New("missing state cookie")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}
	emailTokenCookie, err := r.Cookie(a.cookieAuth.GetSSOEmailTokenCookieName())
	if err != nil {
		a.log.Error("Error verifying SSO auth", zap.Error(console.ErrValidation.New("missing email token cookie")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	ssoState := r.URL.Query().Get("state")
	if ssoState == "" {
		a.log.Error("Error verifying SSO auth", zap.Error(console.ErrValidation.New("missing state value")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	if ssoState != stateCookie.Value {
		a.log.Error("Error verifying SSO auth", zap.Error(sso.ErrInvalidState.New("")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	err = a.service.ValidateSecurityToken(ssoState)
	if err != nil {
		a.log.Error("Error verifying SSO auth", zap.Error(sso.ErrInvalidState.New("invalid signature")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		a.log.Error("Error verifying SSO auth", zap.Error(console.ErrValidation.New("missing auth code")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	claims, err := a.ssoService.VerifySso(ctx, provider, emailTokenCookie.Value, code)
	if err != nil {
		a.log.Error("Error verifying SSO auth", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	a.cookieAuth.RemoveSSOCookies(w)

	ip, err := web.GetRequestIP(r)
	if err != nil {
		a.log.Error("Error getting request IP", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}
	userAgent := r.UserAgent()

	user, err := a.service.GetUserForSsoAuth(ctx, *claims, provider, ip, userAgent)
	if err != nil {
		a.log.Error("Error getting user for sso auth", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	tokenInfo, err := a.service.GenerateSessionToken(ctx, console.SessionTokenRequest{
		UserID:          user.ID,
		TenantID:        user.TenantID,
		Email:           user.Email,
		IP:              ip,
		UserAgent:       userAgent,
		AnonymousID:     LoadAjsAnonymousID(r),
		CustomDuration:  nil,
		HubspotObjectID: user.HubspotObjectID,
	})
	if err != nil {
		a.log.Error("Failed to generate session token", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	a.cookieAuth.SetTokenCookie(w, *tokenInfo)

	http.Redirect(w, r, a.ExternalAddress, http.StatusFound)
}

// GetSsoUrl returns the SSO URL for the given provider.
func (a *Auth) GetSsoUrl(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	provider := a.ssoService.GetProviderByEmail(r.URL.Query().Get("email"))
	if provider == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ssoUrl, err := url.JoinPath(a.ExternalAddress, "sso", provider)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
	_, err = w.Write([]byte(ssoUrl))
	if err != nil {
		a.log.Error("failed to write response", zap.Error(err))
	}
}

// BeginSsoFlow starts the SSO flow by redirecting to the OIDC provider.
func (a *Auth) BeginSsoFlow(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	ssoFailedAddr, err := url.JoinPath(a.ExternalAddress, "login?sso_failed=true")
	if err != nil {
		a.log.Error("failed to get sso failed url", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	provider := mux.Vars(r)["provider"]
	oidcSetup := a.ssoService.GetOidcSetupByProvider(ctx, provider)
	if oidcSetup == nil {
		a.log.Error("invalid provider "+provider, zap.Error(console.ErrValidation.New("invalid provider")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		a.log.Error("email is required for SSO flow", zap.Error(console.ErrValidation.New("email is required")))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	emailToken, err := a.ssoService.GetSsoEmailToken(email)
	if err != nil {
		a.log.Error("failed to get security token", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	state, err := a.csrfService.GenerateSecurityToken()
	if err != nil {
		a.log.Error("failed to generate sso state", zap.Error(err))
		http.Redirect(w, r, ssoFailedAddr, http.StatusPermanentRedirect)
		return
	}

	a.cookieAuth.SetSSOCookies(w, state, emailToken)

	http.Redirect(w, r, oidcSetup.Config.AuthCodeURL(state), http.StatusFound)
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
		InviterEmail     string `json:"inviterEmail"`
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

	if a.MemberAccountsEnabled && registerData.InviterEmail != "" && !utils.ValidateEmail(registerData.InviterEmail) {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("Invalid inviter email.")))
		return
	}

	if a.badPasswords != nil {
		_, exists := a.badPasswords[registerData.Password]
		if exists {
			a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("The password you chose is on a list of insecure or breached passwords. Please choose a different one.")))
			return
		}
	}

	if len([]rune(registerData.Partner)) > 100 {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("Partner must be less than or equal to 100 characters")))
		return
	}

	if len([]rune(registerData.SignupPromoCode)) > 100 {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("Promo code must be less than or equal to 100 characters")))
		return
	}

	ip, err := web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	valid, captchaScore, err := a.service.VerifyRegistrationCaptcha(ctx, registerData.CaptchaResponse, ip)
	if err != nil {
		mon.Counter("create_user_captcha_error").Inc(1)
		a.log.Error("captcha authorization failed", zap.Error(err))

		a.serveJSONError(ctx, w, console.ErrCaptcha.Wrap(err))
		return
	}
	if !valid {
		mon.Counter("create_user_captcha_unsuccessful").Inc(1)

		a.serveJSONError(ctx, w, console.ErrCaptcha.New("captcha validation unsuccessful"))
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

	if registerData.Partner != "" {
		registerData.UserAgent = []byte(registerData.Partner)
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

	requestData := console.CreateUser{
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
		CaptchaScore:     captchaScore,
		IP:               ip,
		SignupPromoCode:  registerData.SignupPromoCode,
		ActivationCode:   code,
		SignupId:         requestID,
		// the minimal signup from the v2 app doesn't require name.
		AllowNoName: registerData.IsMinimal,
	}

	var invitation *console.ProjectInvitation
	if a.MemberAccountsEnabled && registerData.InviterEmail != "" {
		invitation, err = a.handleProjectInvitation(ctx, registerData.Email, registerData.InviterEmail)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		requestData.Kind = console.MemberUser
		requestData.NoTrialExpiration = true
	}

	var user *console.User
	if len(unverified) > 0 {
		user = &unverified[0]

		err = a.service.UpdateUserOnSignup(ctx, user, requestData)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		user.SignupId = requestData.SignupId
		user.ActivationCode = requestData.ActivationCode
	} else {
		secret, err := console.RegistrationSecretFromBase64(registerData.SecretInput)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}

		user, err = a.service.CreateUser(ctx, requestData, secret)
		if err != nil {
			if !console.ErrEmailUsed.Has(err) {
				a.serveJSONError(ctx, w, err)
			}
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
			ID:            user.ID,
			AnonymousID:   LoadAjsAnonymousID(r),
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
		tenantCtx := tenancy.GetContext(ctx)
		if tenantCtx != nil {
			trackCreateUserFields.TenantID = &tenantCtx.TenantID
		}

		a.analytics.TrackCreateUser(trackCreateUserFields)
	}

	if a.MemberAccountsEnabled && invitation != nil {
		a.service.JoinProjectNoAuth(ctx, invitation.ProjectID, user, console.RoleMember)
		a.analytics.TrackInviteLinkSignup(invitation.Email, registerData.Email)
	} else {
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
	}

	if a.ActivationCodeEnabled {
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

func (a *Auth) handleProjectInvitation(ctx context.Context, userEmail, inviterEmail string) (invitation *console.ProjectInvitation, err error) {
	invites, err := a.service.GetInvitesByEmail(ctx, userEmail)
	if err != nil {
		return nil, console.ErrProjectInviteInvalid.New("could not get invitations")
	}
	if len(invites) == 0 {
		return nil, console.ErrProjectInviteInvalid.New("no valid invitation found")
	}

	inviter, _, err := a.service.GetUserByEmailWithUnverified(ctx, inviterEmail)
	if err != nil {
		return nil, console.ErrProjectInviteInvalid.New("error getting inviter info")
	}
	if inviter == nil {
		return nil, console.ErrProjectInviteInvalid.New("could not find inviter")
	}

	for _, invite := range invites {
		if invite.InviterID != nil && *invite.InviterID == inviter.ID {
			invitation = &invite
			break
		}
	}

	if invitation == nil {
		return nil, console.ErrProjectInviteInvalid.New("no valid invitation found")
	}
	if a.service.IsProjectInvitationExpired(invitation) {
		return nil, console.ErrProjectInviteInvalid.New("the invitation has expired")
	}

	proj, err := a.service.GetProjectNoAuth(ctx, invitation.ProjectID)
	if err != nil {
		return nil, console.ErrProjectInviteInvalid.New("could not get project info")
	}
	if proj.Status != nil && *proj.Status == console.ProjectDisabled {
		return nil, console.ErrProjectInviteInvalid.New("the project you were invited to no longer exists")
	}

	return invitation, nil
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

	if len(activateData.Code) != 6 {
		a.serveJSONError(ctx, w, console.ErrValidation.New("the activation code must be 6 characters long"))
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
	for _, u := range unverified {
		if u.Status == console.Inactive {
			u2 := u
			user = &u2
			break
		}
	}
	if user == nil {
		a.serveJSONError(ctx, w, console.ErrEmailNotFound.New("no unverified user found"))
		return
	}

	now := time.Now()

	if user.LoginLockoutExpiration.After(now) {
		a.serveJSONError(ctx, w, console.ErrActivationCode.New("invalid activation code or account locked"))
		return
	}

	if user.ActivationCode != activateData.Code || user.SignupId != activateData.SignupId {
		lockoutDuration, err := a.service.UpdateUsersFailedLoginState(ctx, user)
		if err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}
		if lockoutDuration > 0 {
			a.mailService.SendRenderedAsync(
				ctx,
				[]post.Address{{Address: user.Email, Name: user.FullName}},
				&console.ActivationLockAccountEmail{
					LockoutDuration: lockoutDuration,
					SupportURL:      a.GeneralRequestURL,
				},
			)
		}

		mon.Counter("account_activation_failed").Inc(1)
		mon.IntVal("account_activation_user_failed_count").Observe(int64(user.FailedLoginCount))
		penaltyThreshold := a.service.GetLoginAttemptsWithoutPenalty()

		if user.FailedLoginCount == penaltyThreshold {
			mon.Counter("account_activation_lockout_initiated").Inc(1)
		}

		if user.FailedLoginCount > penaltyThreshold {
			mon.Counter("account_activation_lockout_reinitiated").Inc(1)
		}

		a.serveJSONError(ctx, w, console.ErrActivationCode.New("invalid activation code or account locked"))
		return
	}

	if user.FailedLoginCount != 0 {
		if err := a.service.ResetAccountLock(ctx, user); err != nil {
			a.serveJSONError(ctx, w, err)
			return
		}
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

	tokenInfo, err := a.service.GenerateSessionToken(ctx, console.SessionTokenRequest{
		UserID:          user.ID,
		TenantID:        user.TenantID,
		Email:           user.Email,
		IP:              ip,
		UserAgent:       r.UserAgent(),
		AnonymousID:     LoadAjsAnonymousID(r),
		CustomDuration:  nil,
		HubspotObjectID: user.HubspotObjectID,
	})
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

// LoadAjsAnonymousID looks for ajs_anonymous_id cookie.
// this cookie is set from the website if the user opts into cookies from Storj.
func LoadAjsAnonymousID(req *http.Request) string {
	cookie, err := req.Cookie("ajs_anonymous_id")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// AccountActionData holds data needed to perform change email or account delete actions.
type AccountActionData struct {
	Step console.AccountActionStep `json:"step"`
	Data string                    `json:"data"`
}

// ChangeEmail handles change email flow requests.
func (a *Auth) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data AccountActionData
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	if data.Step < console.VerifyAccountPasswordStep || data.Step > console.VerifyNewAccountEmailStep {
		a.serveJSONError(ctx, w, console.ErrValidation.New("step value is out of range"))
		return
	}

	if data.Data == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("data value can't be empty"))
		return
	}

	if err = a.service.ChangeEmail(ctx, data.Step, data.Data); err != nil {
		a.serveJSONError(ctx, w, err)
	}
}

// DeleteAccount handles self-serve delete account flow requests.
func (a *Auth) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data AccountActionData
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	if data.Step < console.DeleteAccountInit || data.Step > console.DeleteAccountStep {
		a.serveJSONError(ctx, w, console.ErrValidation.New("step value is out of range"))
		return
	}

	if data.Step > console.DeleteAccountInit && data.Step != console.DeleteAccountStep && data.Data == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("data value can't be empty"))
		return
	}

	resp, err := a.service.DeleteAccount(ctx, data.Step, data.Data)
	if err != nil {
		a.serveJSONError(ctx, w, err)
	}

	if resp != nil {
		w.WriteHeader(http.StatusConflict)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			a.log.Error("could not encode account deletion response", zap.Error(ErrAuthAPI.Wrap(err)))
		}
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

// GetBadPasswords returns a list of encoded bad passwords.
func (a *Auth) GetBadPasswords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Cache-Control", "public, max-age=604800") // cache response for 7 days.
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Content-Disposition", "attachment; filename=\"bad-passwords.txt\"")

	if _, err = w.Write([]byte(a.badPasswordsEncoded)); err != nil {
		a.log.Error("could not write encoded bad passwords", zap.Error(ErrAuthAPI.Wrap(err)))
	}
}

// GetAccount gets authorized user and take it's params.
func (a *Auth) GetAccount(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	consoleUser, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	freezes, err := a.accountFreezeService.GetAll(ctx, consoleUser.ID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	user := console.UserAccount{
		FreezeStatus: console.FreezeStat{
			Frozen:             freezes.BillingFreeze != nil,
			Warned:             freezes.BillingWarning != nil,
			TrialExpiredFrozen: freezes.TrialExpirationFreeze != nil,
		},
	}
	if user.FreezeStatus.TrialExpiredFrozen {
		days := a.accountFreezeService.GetDaysTillEscalation(*freezes.TrialExpirationFreeze, time.Now())
		if days != nil && *days > 0 {
			user.FreezeStatus.TrialExpirationGracePeriod = *days
		}
	}

	user.ShortName = consoleUser.ShortName
	user.FullName = consoleUser.FullName
	user.Email = consoleUser.Email
	user.ID = consoleUser.ID
	if consoleUser.ExternalID != nil {
		user.ExternalID = *consoleUser.ExternalID
	}
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
	user.PaidTier = consoleUser.IsPaid()
	user.Kind = consoleUser.Kind.Info()
	user.MFAEnabled = consoleUser.MFAEnabled
	user.MFARecoveryCodeCount = len(consoleUser.MFARecoveryCodes)
	user.CreatedAt = consoleUser.CreatedAt
	user.PendingVerification = consoleUser.Status == console.PendingBotVerification
	user.TrialExpiration = consoleUser.TrialExpiration
	user.HasVarPartner, err = a.service.GetUserHasVarPartner(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(&user)
	if err != nil {
		a.log.Error("could not encode user info", zap.Error(ErrAuthAPI.Wrap(err)))
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
		a.serveJSONError(ctx, w, err)
		return
	}

	if a.badPasswords != nil {
		_, exists := a.badPasswords[passwordChange.NewPassword]
		if exists {
			a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("The password you chose is on a list of insecure or breached passwords. Please choose a different one.")))
			return
		}
	}

	sessionID, err := a.getSessionID(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	err = a.service.ChangePassword(ctx, passwordChange.CurrentPassword, passwordChange.NewPassword, &sessionID)
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

	recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, user)
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

	var resendEmail struct {
		Email           string `json:"email"`
		CaptchaResponse string `json:"captchaResponse"`
	}

	err = json.NewDecoder(r.Body).Decode(&resendEmail)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	ip, err := web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	valid, _, err := a.service.VerifyRegistrationCaptcha(ctx, resendEmail.CaptchaResponse, ip)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}
	if !valid {
		a.serveJSONError(ctx, w, console.ErrCaptcha.New("captcha validation unsuccessful"))
		return
	}

	verified, unverified, err := a.service.GetUserByEmailWithUnverified(ctx, resendEmail.Email)
	if err != nil {
		return
	}

	if verified != nil {
		recoveryToken, err := a.service.GeneratePasswordRecoveryToken(ctx, verified)
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
		a.log.Error("could not delete all other sessions", zap.Error(ErrAuthAPI.Wrap(err)))
	}

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

	if a.badPasswords != nil {
		_, exists := a.badPasswords[resetPassword.NewPassword]
		if exists {
			a.serveJSONError(ctx, w, console.ErrValidation.Wrap(errs.New("The password you chose is on a list of insecure or breached passwords. Please choose a different one.")))
			return
		}
	}

	err = a.service.ResetPassword(ctx, resetPassword.RecoveryToken, resetPassword.NewPassword, resetPassword.MFAPasscode, resetPassword.MFARecoveryCode, time.Now())

	if console.ErrTooManyAttempts.Has(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(a.getStatusCode(err))

		err = json.NewEncoder(w).Encode(map[string]string{
			"error": a.getUserErrorMessage(err),
			"code":  "too_many_attempts",
		})

		if err != nil {
			a.log.Error("failed to write json response", zap.Error(ErrUtils.Wrap(err)))
		}

		return
	}

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

// GetActiveSessions gets user's active sessions.
func (a *Auth) GetActiveSessions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	query := r.URL.Query()

	limitParam := query.Get("limit")
	if limitParam == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("parameter 'limit' can't be empty"))
		return
	}

	limit, err := strconv.ParseUint(limitParam, 10, 32)
	if err != nil {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(err))
		return
	}

	pageParam := query.Get("page")
	if pageParam == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("parameter 'page' can't be empty"))
		return
	}

	page, err := strconv.ParseUint(pageParam, 10, 32)
	if err != nil {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(err))
		return
	}

	orderParam := query.Get("order")
	if orderParam == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("parameter 'order' can't be empty"))
		return
	}

	order, err := strconv.ParseUint(orderParam, 10, 32)
	if err != nil {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(err))
		return
	}

	orderDirectionParam := query.Get("orderDirection")
	if orderDirectionParam == "" {
		a.serveJSONError(ctx, w, console.ErrValidation.New("parameter 'orderDirection' can't be empty"))
		return
	}

	orderDirection, err := strconv.ParseUint(orderDirectionParam, 10, 32)
	if err != nil {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(err))
		return
	}

	cursor := consoleauth.WebappSessionsCursor{
		Limit:          uint(limit),
		Page:           uint(page),
		Order:          consoleauth.WebappSessionsOrder(order),
		OrderDirection: consoleauth.OrderDirection(orderDirection),
	}

	sessionsPage, err := a.service.GetPagedActiveSessions(ctx, cursor)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	currentSessionID, err := a.getSessionID(r)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	for i, session := range sessionsPage.Sessions {
		if session.ID == currentSessionID {
			sessionsPage.Sessions[i].IsRequesterCurrentSession = true
			break
		}
	}

	err = json.NewEncoder(w).Encode(sessionsPage)
	if err != nil {
		a.log.Error("failed to write json paged active webapp sessions response", zap.Error(ErrAuthAPI.Wrap(err)))
	}
}

// InvalidateSessionByID invalidates user session by ID.
func (a *Auth) InvalidateSessionByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	sessionIDStr, ok := mux.Vars(r)["id"]
	if !ok {
		a.serveJSONError(ctx, w, console.ErrValidation.New("id parameter is missing"))
		return
	}

	sessionID, err := uuid.FromString(sessionIDStr)
	if err != nil {
		a.serveJSONError(ctx, w, console.ErrValidation.Wrap(err))
		return
	}

	err = a.service.InvalidateSession(ctx, sessionID)
	if err != nil {
		a.serveJSONError(ctx, w, err)
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
		OnboardingStart  *bool                    `json:"onboardingStart"`
		OnboardingEnd    *bool                    `json:"onboardingEnd"`
		PassphrasePrompt *bool                    `json:"passphrasePrompt"`
		OnboardingStep   *string                  `json:"onboardingStep"`
		SessionDuration  *int64                   `json:"sessionDuration"`
		NoticeDismissal  *console.NoticeDismissal `json:"noticeDismissal"`
	}

	err = json.NewDecoder(r.Body).Decode(&updateInfo)
	if err != nil {
		a.serveJSONError(ctx, w, err)
		return
	}

	if updateInfo.NoticeDismissal != nil && len(updateInfo.NoticeDismissal.Announcements) > 0 {
		filteredAnnouncements := make(map[string]bool)
		for announcement, dismissed := range updateInfo.NoticeDismissal.Announcements {
			if announcement == "" {
				// Skip storing dismissal for empty string announcements (not permanently dismissible).
				continue
			}
			if !slices.Contains(a.validAnnouncementNames, announcement) {
				a.log.Error("invalid announcement name in notice dismissal", zap.String("name", announcement))
				continue
			}

			filteredAnnouncements[announcement] = dismissed
		}

		updateInfo.NoticeDismissal.Announcements = filteredAnnouncements
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
		NoticeDismissal:  updateInfo.NoticeDismissal,
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
	case console.ErrValidation.Has(err), console.ErrCaptcha.Has(err),
		console.ErrMFAMissing.Has(err), console.ErrMFAPasscode.Has(err),
		console.ErrMFARecoveryCode.Has(err), console.ErrChangePassword.Has(err),
		console.ErrInvalidProjectLimit.Has(err), sso.ErrInvalidProvider.Has(err),
		sso.ErrInvalidCode.Has(err), sso.ErrNoIdToken.Has(err):
		return http.StatusBadRequest
	case console.ErrUnauthorized.Has(err), console.ErrTokenExpiration.Has(err),
		console.ErrRecoveryToken.Has(err), console.ErrLoginCredentials.Has(err),
		console.ErrActivationCode.Has(err), sso.ErrTokenVerification.Has(err),
		sso.ErrInvalidState.Has(err):
		return http.StatusUnauthorized
	case console.ErrEmailUsed.Has(err), console.ErrMFAConflict.Has(err), console.ErrMFAEnabled.Has(err), console.ErrConflict.Has(err):
		return http.StatusConflict
	case console.ErrLoginRestricted.Has(err), console.ErrTooManyAttempts.Has(err), console.ErrForbidden.Has(err), console.ErrSsoUserRestricted.Has(err), console.ErrProjectInviteInvalid.Has(err):
		return http.StatusForbidden
	case errors.Is(err, errNotImplemented):
		return http.StatusNotImplemented
	case console.ErrNotPaidTier.Has(err):
		return http.StatusPaymentRequired
	case errors.As(err, &maxBytesError):
		return http.StatusRequestEntityTooLarge
	case console.ErrEmailNotFound.Has(err):
		return http.StatusNotFound
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
	case console.ErrLoginRestricted.Has(err):
		return "You can't be authenticated. Please contact support"
	case console.ErrValidation.Has(err), console.ErrChangePassword.Has(err), console.ErrInvalidProjectLimit.Has(err),
		console.ErrNotPaidTier.Has(err), console.ErrTooManyAttempts.Has(err), console.ErrMFAEnabled.Has(err),
		console.ErrForbidden.Has(err), console.ErrConflict.Has(err), console.ErrProjectInviteInvalid.Has(err):
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
