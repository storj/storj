// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	mathrand "math/rand"
	"net/http"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slices"

	"storj.io/common/cfgstruct"
	"storj.io/common/currency"
	"storj.io/common/http/requestid"
	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/useragent"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/private/blockchain"
	"storj.io/storj/private/post"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleauth/sso"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
	"storj.io/storj/satellite/console/restapikeys"
	"storj.io/storj/satellite/console/valdi"
	"storj.io/storj/satellite/console/valdi/valdiclient"
	"storj.io/storj/satellite/emission"
	"storj.io/storj/satellite/entitlements"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/hubspotmails"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/payments/billing"
	"storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/satellite/tenancy"
)

var mon = monkit.Package()

const (
	// maxLimit specifies the limit for all paged queries.
	maxLimit = 300

	// TestPasswordCost is the hashing complexity to use for testing.
	TestPasswordCost = bcrypt.MinCost

	// hoursPerMonth is the number of hours in a month.
	hoursPerMonth = 24 * 30
)

// Error messages.
const (
	unauthorizedErrMsg                   = "You are not authorized to perform this action"
	emailUsedErrMsg                      = "This email is already in use, try another"
	emailNotFoundErrMsg                  = "There are no users with the specified email"
	passwordRecoveryTokenIsExpiredErrMsg = "Your password recovery link has expired, please request another one"
	credentialsErrMsg                    = "Your login credentials are incorrect, please try again"
	tooManyAttemptsErrMsg                = "Too many attempts, please try again later"
	generateSessionTokenErrMsg           = "Failed to generate session token"
	failedToRetrieveUserErrMsg           = "Failed to retrieve user from database"
	apiKeyCredentialsErrMsg              = "Your API Key is incorrect"
	changePasswordErrMsg                 = "Your old password is incorrect, please try again"
	passwordTooShortErrMsg               = "Your password needs to be at least %d characters long"
	passwordTooLongErrMsg                = "Your password must be no longer than %d characters"
	projectOwnerDeletionForbiddenErrMsg  = "%s is a project owner and can not be deleted"
	apiKeyWithNameExistsErrMsg           = "An API Key with this name already exists in this project, please use a different name"
	apiKeyWithNameDoesntExistErrMsg      = "An API Key with this name doesn't exist in this project."
	teamMemberDoesNotExistErrMsg         = "There are no team members with the email '%s'. Please try again."
	activationTokenExpiredErrMsg         = "This activation token has expired, please request another one"
	usedRegTokenErrMsg                   = "This registration token has already been used"
	projLimitErrMsg                      = "Sorry, project creation is limited for your account. Please contact support!"
	projNameErrMsg                       = "The new project must have a name you haven't used before!"
	projInviteInvalidErrMsg              = "The invitation has expired or is invalid"
	projInviterInvalidErrMsg             = "The inviter is no longer part of the project"
	projInviteAlreadyMemberErrMsg        = "You are already a member of the project"
	projInviteResponseInvalidErrMsg      = "Invalid project member invitation response"
	activeProjInviteExistsErrMsg         = "An active invitation for '%s' already exists"
	projInviteExistsErrMsg               = "An invitation for '%s' already exists"
	projInviteDoesntExistErrMsg          = "An invitation for '%s' does not exist"
	contactSupportErrMsg                 = "Please contact support"
	accountActionWrongStepOrderErrMsg    = "Wrong step order. Please restart the flow"
)

var (
	// Error describes internal console error.
	Error = errs.Class("console service")

	// ErrUnauthorized is error class for authorization related errors.
	ErrUnauthorized = errs.Class("unauthorized")

	// ErrNoMembership is error type of not belonging to a specific project.
	ErrNoMembership = errs.Class("no membership")

	// ErrTokenExpiration is error type of token reached expiration time.
	ErrTokenExpiration = errs.Class("token expiration")

	// ErrTokenInvalid is error type of tokens which are invalid.
	ErrTokenInvalid = errs.Class("invalid token")

	// ErrProjLimit is error type of project limit.
	ErrProjLimit = errs.Class("project limit")

	// ErrUsage is error type of project usage.
	ErrUsage = errs.Class("project usage")

	// ErrLoginCredentials occurs when provided invalid login credentials.
	ErrLoginCredentials = errs.Class("login credentials")

	// ErrSsoUserRestricted occurs when an SSO user attempts an action they are restricted from.
	ErrSsoUserRestricted = errs.Class("SSO user restricted")

	// ErrTooManyAttempts occurs when user tries to produce auth-related action too many times.
	ErrTooManyAttempts = errs.Class("too many attempts")

	// ErrActivationCode is error class for failed signup code activation.
	ErrActivationCode = errs.Class("activation code")

	// ErrChangePassword occurs when provided old password is incorrect.
	ErrChangePassword = errs.Class("change password")

	// ErrEmailUsed is error type that occurs on repeating auth attempts with email.
	ErrEmailUsed = errs.Class("email used")

	// ErrEmailNotFound occurs when no users have the specified email.
	ErrEmailNotFound = errs.Class("email not found")

	// ErrExternalIdNotFound occurs when no users have the specified external ID.
	ErrExternalIdNotFound = errs.Class("external ID not found")

	// ErrNoAPIKey is error type that occurs when there is no api key found.
	ErrNoAPIKey = errs.Class("no api key found")

	// ErrAPIKeyRequest is returned when there is an error parsing a request for api keys.
	ErrAPIKeyRequest = errs.Class("api key request")

	// ErrRegToken describes registration token errors.
	ErrRegToken = errs.Class("registration token")

	// ErrCaptcha describes captcha validation errors.
	ErrCaptcha = errs.Class("captcha validation")

	// ErrRecoveryToken describes account recovery token errors.
	ErrRecoveryToken = errs.Class("recovery token")

	// ErrProjName is error that occurs with reused project names.
	ErrProjName = errs.Class("project name")

	// ErrPurchaseDesc is error that occurs when something is wrong with Purchase description.
	ErrPurchaseDesc = errs.Class("purchase description")

	// ErrAlreadyHasPackage is error that occurs when a user tries to update package, but already has one.
	ErrAlreadyHasPackage = errs.Class("user already has package")

	// ErrAlreadyMember occurs when a user tries to reject an invitation to a project they're already a member of.
	ErrAlreadyMember = errs.Class("already a member")

	// ErrProjectInviteInvalid occurs when a user tries to act upon an invitation that doesn't exist
	// or has expired.
	ErrProjectInviteInvalid = errs.Class("invalid project invitation")

	// ErrConflict occurs when a user attempts an operation that conflicts with the current state.
	ErrConflict = errs.Class("conflict detected")

	// ErrNotFound occurs when a user attempts an operation that references a resource that does not exist.
	ErrNotFound = errs.Class("not found")

	// ErrSatelliteManagedEncryption occurs when a user attempts to create a satellite managed
	// encryption project when it is disabled.
	ErrSatelliteManagedEncryption = ErrConflict.New("satellite managed encryption is not enabled")

	// ErrForbidden occurs when a user attempts an operation without sufficient access rights.
	ErrForbidden = errs.Class("insufficient access rights")

	// ErrAlreadyInvited occurs when trying to invite a user who has already been invited.
	ErrAlreadyInvited = errs.Class("user is already invited")

	// ErrInvalidProjectLimit occurs when the requested project limit is not a non-negative integer and/or greater than the current project limit.
	ErrInvalidProjectLimit = errs.Class("requested project limit is invalid")

	// ErrNotPaidTier occurs when a user must be paid tier in order to complete an operation.
	ErrNotPaidTier = errs.Class("user is not paid tier")

	// ErrBotUser occurs when a user must be verified by admin first in order to complete operation.
	ErrBotUser = errs.Class("user has to be verified by admin first")

	// ErrLoginRestricted occurs when a user with PendingBotVerification or LegalHold status tries to log in.
	ErrLoginRestricted = errs.Class("user can't be authenticated")

	// ErrFailedToUpgrade occurs when a user can't be upgraded to paid tier.
	ErrFailedToUpgrade = errs.Class("failed to upgrade user to paid tier")

	// ErrPlacementNotFound occurs when a placement is not found.
	ErrPlacementNotFound = errs.Class("placement not found")

	// ErrInvalidKey is an error type that occurs when a user submits an API key
	// that does not match anything in the database.
	ErrInvalidKey = errs.Class("invalid key")
)

// Service is handling accounts related logic.
//
// architecture: Service
type Service struct {
	log, auditLogger           *zap.Logger
	store                      DB
	restKeys                   restapikeys.DB
	oauthRestKeys              restapikeys.Service
	projectAccounting          accounting.ProjectAccounting
	projectUsage               *accounting.Service
	buckets                    buckets.DB
	attributions               attribution.DB
	placements                 nodeselection.PlacementDefinitions
	placementNameLookup        map[string]storj.PlacementConstraint
	placementProductMap        map[int]int32
	productConfigs             map[int32]payments.ProductUsagePriceModel
	accounts                   payments.Accounts
	depositWallets             payments.DepositWallets
	billing                    billing.TransactionsDB
	registrationCaptchaHandler CaptchaHandler
	loginCaptchaHandler        CaptchaHandler
	analytics                  *analytics.Service
	tokens                     *consoleauth.Service
	mailService                *mailservice.Service
	hubspotMailService         *hubspotmails.Service
	accountFreezeService       *AccountFreezeService
	emission                   *emission.Service
	kmsService                 *kms.Service
	ssoService                 *sso.Service
	valdiService               *valdi.Service

	satelliteAddress string
	satelliteName    string

	config            Config
	maxProjectBuckets int
	ssoEnabled        bool

	varPartners             map[string]struct{}
	auditableAPIKeyProjects map[string]struct{}

	paymentSourceChainIDs map[int64]string

	objectLockAndVersioningConfig ObjectLockAndVersioningConfig

	entitlementsService *entitlements.Service
	entitlementsConfig  entitlements.Config

	minimumChargeAmount int64
	minimumChargeDate   *time.Time

	packagePlans map[string]payments.PackagePlan

	legacyPlacements []storj.PlacementConstraint

	nowFn func() time.Time
}

func init() {
	var c Config
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &c, cfgstruct.UseTestDefaults())
	if c.PasswordCost != TestPasswordCost {
		panic("invalid test constant defined in struct tag")
	}
	cfgstruct.Bind(pflag.NewFlagSet("", pflag.PanicOnError), &c, cfgstruct.UseReleaseDefaults())
	if c.PasswordCost != 0 {
		panic("invalid release constant defined in struct tag. should be 0 (=automatic)")
	}

	for _, id := range c.Placement.AllowedPlacementIdsForNewProjects {
		if _, ok := c.Placement.SelfServeDetails.Get(id); !ok {
			panic(fmt.Sprintf("allowed placement ID %d not found in self-serve placement details", id))
		}
	}

	for _, id := range c.LegacyPlacements {
		if _, err := strconv.ParseUint(id, 0, 16); err != nil {
			panic(fmt.Sprintf("invalid legacy placement ID: %s", id))
		}
	}
}

// Payments separates all payment related functionality.
type Payments struct {
	service *Service
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, store DB, restKeys restapikeys.DB, oauthRestKeys restapikeys.Service, projectAccounting accounting.ProjectAccounting,
	projectUsage *accounting.Service, buckets buckets.DB, attributions attribution.DB, accounts payments.Accounts, depositWallets payments.DepositWallets,
	billingDb billing.TransactionsDB, analytics *analytics.Service, tokens *consoleauth.Service, mailService *mailservice.Service, hubspotMailService *hubspotmails.Service,
	accountFreezeService *AccountFreezeService, emission *emission.Service, kmsService *kms.Service, ssoService *sso.Service, satelliteAddress string,
	satelliteName string, maxProjectBuckets int, ssoEnabled bool, placements nodeselection.PlacementDefinitions,
	objectLockAndVersioningConfig ObjectLockAndVersioningConfig, valdiService *valdi.Service, minimumChargeAmount int64,
	minimumChargeDate *time.Time, packagePlans map[string]payments.PackagePlan, entitlementsConfig entitlements.Config,
	entitlementsService *entitlements.Service, placementProductMap map[int]int32, productConfigs map[int32]payments.ProductUsagePriceModel, config Config) (*Service, error) {
	if store == nil {
		return nil, errs.New("store can't be nil")
	}
	if log == nil {
		return nil, errs.New("log can't be nil")
	}
	if config.PasswordCost == 0 {
		config.PasswordCost = bcrypt.DefaultCost
	}

	// We have two separate captcha handlers for login and registration.
	// We want to easily swap between captchas independently.
	// For example, google recaptcha for login screen and hcaptcha for registration screen.
	var registrationCaptchaHandler CaptchaHandler
	if config.Captcha.Registration.Recaptcha.Enabled {
		registrationCaptchaHandler = NewDefaultCaptcha(Recaptcha, config.Captcha.Registration.Recaptcha.SecretKey)
	} else if config.Captcha.Registration.Hcaptcha.Enabled {
		registrationCaptchaHandler = NewDefaultCaptcha(Hcaptcha, config.Captcha.Registration.Hcaptcha.SecretKey)
	}

	var loginCaptchaHandler CaptchaHandler
	if config.Captcha.Login.Recaptcha.Enabled {
		loginCaptchaHandler = NewDefaultCaptcha(Recaptcha, config.Captcha.Login.Recaptcha.SecretKey)
	} else if config.Captcha.Login.Hcaptcha.Enabled {
		loginCaptchaHandler = NewDefaultCaptcha(Hcaptcha, config.Captcha.Login.Hcaptcha.SecretKey)
	}

	partners := make(map[string]struct{}, len(config.VarPartners))
	for _, partner := range config.VarPartners {
		partners[partner] = struct{}{}
	}

	paymentSourceChainIDs := make(map[int64]string)
	for source, IDs := range billing.SourceChainIDs {
		for _, ID := range IDs {
			paymentSourceChainIDs[ID] = source
		}
	}

	placementNameLookup := make(map[string]storj.PlacementConstraint, len(placements))
	for _, placement := range placements {
		placementNameLookup[placement.Name] = placement.ID
	}

	auditableAPIKeyProjects := make(map[string]struct{}, len(config.AuditableAPIKeyProjects))
	for _, projectID := range config.AuditableAPIKeyProjects {
		auditableAPIKeyProjects[projectID] = struct{}{}
	}

	var legacyPlacements []storj.PlacementConstraint
	for _, id := range config.LegacyPlacements {
		parsed, err := strconv.ParseUint(id, 0, 16)
		if err != nil {
			return nil, errs.New("invalid legacy placement ID: %s", id)
		}

		legacyPlacements = append(legacyPlacements, storj.PlacementConstraint(parsed))
	}

	return &Service{
		log:                           log,
		auditLogger:                   log.Named("auditlog"),
		store:                         store,
		restKeys:                      restKeys,
		oauthRestKeys:                 oauthRestKeys,
		projectAccounting:             projectAccounting,
		projectUsage:                  projectUsage,
		buckets:                       buckets,
		attributions:                  attributions,
		placements:                    placements,
		placementNameLookup:           placementNameLookup,
		placementProductMap:           placementProductMap,
		productConfigs:                productConfigs,
		accounts:                      accounts,
		depositWallets:                depositWallets,
		billing:                       billingDb,
		registrationCaptchaHandler:    registrationCaptchaHandler,
		loginCaptchaHandler:           loginCaptchaHandler,
		analytics:                     analytics,
		tokens:                        tokens,
		mailService:                   mailService,
		hubspotMailService:            hubspotMailService,
		accountFreezeService:          accountFreezeService,
		emission:                      emission,
		kmsService:                    kmsService,
		valdiService:                  valdiService,
		ssoService:                    ssoService,
		satelliteAddress:              satelliteAddress,
		satelliteName:                 satelliteName,
		maxProjectBuckets:             maxProjectBuckets,
		ssoEnabled:                    ssoEnabled,
		config:                        config,
		varPartners:                   partners,
		objectLockAndVersioningConfig: objectLockAndVersioningConfig,
		paymentSourceChainIDs:         paymentSourceChainIDs,

		minimumChargeAmount: minimumChargeAmount,
		minimumChargeDate:   minimumChargeDate,
		packagePlans:        packagePlans,

		entitlementsService: entitlementsService,
		entitlementsConfig:  entitlementsConfig,

		legacyPlacements: legacyPlacements,

		nowFn: time.Now,
	}, nil
}

func getRequestingIP(ctx context.Context) (source, forwardedFor string) {
	if req := GetRequest(ctx); req != nil {
		return req.RemoteAddr, req.Header.Get("X-Forwarded-For")
	}
	return "", ""
}

func (s *Service) auditLog(ctx context.Context, operation string, userID *uuid.UUID, email string, extra ...zap.Field) {
	sourceIP, forwardedForIP := getRequestingIP(ctx)
	fields := append(
		make([]zap.Field, 0, len(extra)+6),
		zap.String("operation", operation),
		zap.String("source-ip", sourceIP),
		zap.String("forwarded-for-ip", forwardedForIP),
	)
	if userID != nil {
		fields = append(fields, zap.String("userID", userID.String()))
	}
	if email != "" {
		fields = append(fields, zap.String("email", email))
	}
	if requestID := requestid.FromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("requestID", requestID))
	}

	fields = append(fields, extra...)
	s.auditLogger.Info("console activity", fields...)
}

func (s *Service) getUserAndAuditLog(ctx context.Context, operation string, extra ...zap.Field) (*User, error) {
	user, err := GetUser(ctx)
	if err != nil {
		sourceIP, forwardedForIP := getRequestingIP(ctx)
		s.auditLogger.Info("console activity unauthorized",
			append(append(
				make([]zap.Field, 0, len(extra)+4),
				zap.String("operation", operation),
				zap.Error(err),
				zap.String("source-ip", sourceIP),
				zap.String("forwarded-for-ip", forwardedForIP),
			), extra...)...)
		return nil, err
	}
	s.auditLog(ctx, operation, &user.ID, user.Email, extra...)
	return user, nil
}

// Payments separates all payment related functionality.
func (s *Service) Payments() Payments {
	return Payments{service: s}
}

// GetValdiAPIKey gets a valdi API key. If one doesn't exist, it is created. If a valdi user needs to be created first, it creates that too.
func (s *Service) GetValdiAPIKey(ctx context.Context, projectID uuid.UUID) (key *valdiclient.CreateAPIKeyResponse, status int, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get valdi api key", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, http.StatusInternalServerError, Error.Wrap(err)
	}

	// TODO: all project members?
	_, p, err := s.isProjectOwner(ctx, user.ID, projectID)
	if err != nil {
		status = http.StatusInternalServerError
		if ErrUnauthorized.Has(err) || errs.Is(err, sql.ErrNoRows) {
			status = http.StatusUnauthorized
		}
		return nil, status, Error.Wrap(err)
	}

	// shouldn't be nil if err is nil, but just check it anyway
	if p == nil {
		return nil, http.StatusInternalServerError, Error.Wrap(errs.New("nil project"))
	}

	key, status, err = s.valdiService.CreateAPIKey(ctx, p.PublicID)
	if status != http.StatusNotFound {
		return key, status, Error.Wrap(err)
	}

	status, err = s.valdiService.CreateUser(ctx, p.PublicID)
	if err != nil {
		return nil, status, Error.Wrap(err)
	}

	key, status, err = s.valdiService.CreateAPIKey(ctx, p.PublicID)
	return key, status, Error.Wrap(err)
}

// StartFreeTrial starts free trial for authorized Member user.
func (payment Payments) StartFreeTrial(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "start free trial")
	if err != nil {
		return Error.Wrap(err)
	}

	if !user.IsMember() {
		return ErrUnauthorized.New("only Member users can start new free trial")
	}

	freeKind := FreeUser
	request := UpdateUserRequest{
		Kind: &freeKind,
	}
	if payment.service.config.FreeTrialDuration != 0 {
		expiration := payment.service.nowFn().Add(payment.service.config.FreeTrialDuration)
		expirationPtr := &expiration
		request.TrialExpiration = &expirationPtr
	}

	return payment.service.store.Users().Update(ctx, user.ID, request)
}

// SetupAccount creates payment account for authorized user.
func (payment Payments) SetupAccount(ctx context.Context) (_ payments.CouponType, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "setup payment account")
	if err != nil {
		return payments.NoCoupon, Error.Wrap(err)
	}

	return payment.service.accounts.Setup(ctx, user.ID, user.Email, user.SignupPromoCode)
}

// ChangeEmail changes payment account's email address.
func (payment Payments) ChangeEmail(ctx context.Context, userID uuid.UUID, email string) (err error) {
	defer mon.Task()(&ctx)(&err)

	return payment.service.accounts.ChangeEmail(ctx, userID, email)
}

// SaveBillingAddress saves billing address for a user and returns the updated billing information.
func (payment Payments) SaveBillingAddress(ctx context.Context, address payments.BillingAddress) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "save billing information")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	newInfo, err := payment.service.accounts.SaveBillingAddress(ctx, "", user.ID, address)

	return newInfo, Error.Wrap(err)
}

// AddInvoiceReference adds a new default invoice reference to be displayed on each invoice and returns the updated billing information.
func (payment Payments) AddInvoiceReference(ctx context.Context, reference string) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "add invoice reference")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	newInfo, err := payment.service.accounts.AddDefaultInvoiceReference(ctx, user.ID, reference)

	return newInfo, Error.Wrap(err)
}

// AddTaxID adds a new tax ID for a user and returns the updated billing information.
func (payment Payments) AddTaxID(ctx context.Context, params payments.AddTaxParams) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "add tax ID")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	newInfo, err := payment.service.accounts.AddTaxID(ctx, "", user.ID, params)

	return newInfo, Error.Wrap(err)
}

// RemoveTaxID removes a tax ID from a user and returns the updated billing information.
func (payment Payments) RemoveTaxID(ctx context.Context, id string) (_ *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "remove tax ID")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	newInfo, err := payment.service.accounts.RemoveTaxID(ctx, user.ID, id)

	return newInfo, Error.Wrap(err)
}

// GetBillingInformation gets a user's billing information.
func (payment Payments) GetBillingInformation(ctx context.Context) (information *payments.BillingInformation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "get billing information")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	information, err = payment.service.accounts.GetBillingInformation(ctx, user.ID)

	return information, Error.Wrap(err)
}

// AccountBalance return account balance.
func (payment Payments) AccountBalance(ctx context.Context) (balance payments.Balance, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "get account balance")
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	err = payment.service.accounts.EnsureUserHasCustomer(ctx, user.ID, user.Email, user.SignupPromoCode)
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	return payment.service.accounts.Balances().Get(ctx, user.ID)
}

// AddCreditCard is used to save new credit card and attach it to payment account.
// TODO: this method should be removed/reworked as it's used only in tests to upgrade users or add mocked cards.
func (payment Payments) AddCreditCard(ctx context.Context, creditCardToken string) (card payments.CreditCard, err error) {
	defer mon.Task()(&ctx, creditCardToken)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "add credit card")
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	err = payment.service.accounts.EnsureUserHasCustomer(ctx, user.ID, user.Email, user.SignupPromoCode)
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	card, err = payment.service.accounts.CreditCards().Add(ctx, user.ID, creditCardToken)
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	payment.service.analytics.TrackCreditCardAdded(user.ID, user.Email, user.HubspotObjectID)

	if user.IsFreeOrMember() {
		err = payment.upgradeToPaidTier(ctx, user)
		if err != nil {
			return payments.CreditCard{}, ErrFailedToUpgrade.Wrap(err)
		}

		payment.service.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email}},
			&UpgradeToProEmail{LoginURL: payment.service.config.LoginURL},
		)
		return card, nil
	}

	payment.service.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email}},
		&CreditCardAddedEmail{
			SupportURL: payment.service.config.SupportURL,
			LoginURL:   payment.service.config.LoginURL,
		},
	)

	return card, nil
}

// UpdateCreditCard is used to update credit card details.
func (payment Payments) UpdateCreditCard(ctx context.Context, params payments.CardUpdateParams) (err error) {
	defer mon.Task()(&ctx, params.CardID)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "update credit card")
	if err != nil {
		return Error.Wrap(err)
	}

	err = payment.service.accounts.CreditCards().Update(ctx, user.ID, params)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// AddCardByPaymentMethodID is used to save new credit card and attach it to payment account.
func (payment Payments) AddCardByPaymentMethodID(ctx context.Context, params *payments.AddCardParams, force bool) (card payments.CreditCard, err error) {
	defer mon.Task()(&ctx)(&err)

	// Unlikely to happen, but just in case.
	if params == nil {
		return payments.CreditCard{}, Error.New("card params are empty")
	}

	user, err := payment.service.getUserAndAuditLog(ctx, "add card by payment method ID")
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	err = payment.service.accounts.EnsureUserHasCustomer(ctx, user.ID, user.Email, user.SignupPromoCode)
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	err = payment.updateCustomerBillingInfo(ctx, user.ID, params.Address, params.Tax)
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	card, err = payment.service.accounts.CreditCards().AddByPaymentMethodID(ctx, user.ID, params.Token, force)
	if err != nil {
		return payments.CreditCard{}, Error.Wrap(err)
	}

	payment.service.analytics.TrackCreditCardAdded(user.ID, user.Email, user.HubspotObjectID)

	if user.IsFreeOrMember() && payment.service.config.UpgradePayUpfrontAmount == 0 {
		err = payment.upgradeToPaidTier(ctx, user)
		if err != nil {
			return payments.CreditCard{}, err
		}

		payment.service.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email}},
			&UpgradeToProEmail{LoginURL: payment.service.config.LoginURL},
		)
		return card, nil
	}

	payment.service.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email}},
		&CreditCardAddedEmail{
			SupportURL: payment.service.config.SupportURL,
			LoginURL:   payment.service.config.LoginURL,
		},
	)

	return card, nil
}

func (payment Payments) upgradeToPaidTier(ctx context.Context, user *User) (err error) {
	// put this user into the paid tier and convert projects to upgraded limits.
	now := payment.service.nowFn()

	freeze, err := payment.service.accountFreezeService.Get(ctx, user.ID, TrialExpirationFreeze)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return Error.Wrap(err)
		}
	}
	if freeze != nil {
		err = payment.service.accountFreezeService.TrialExpirationUnfreezeUser(ctx, user.ID)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	err = payment.service.store.Users().UpdatePaidTier(ctx, user.ID, true,
		payment.service.config.UsageLimits.Bandwidth.Paid,
		payment.service.config.UsageLimits.Storage.Paid,
		payment.service.config.UsageLimits.Segment.Paid,
		payment.service.config.UsageLimits.Project.Paid,
		&now,
	)
	if err != nil {
		return Error.Wrap(err)
	}
	payment.service.analytics.TrackUserUpgraded(user.ID, user.Email, user.TrialExpiration, user.HubspotObjectID)

	projects, err := payment.service.store.Projects().GetOwn(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}
	for _, project := range projects {
		if project.StorageLimit == nil || *project.StorageLimit < payment.service.config.UsageLimits.Storage.Paid {
			project.StorageLimit = new(memory.Size)
			*project.StorageLimit = payment.service.config.UsageLimits.Storage.Paid
		}
		if project.BandwidthLimit == nil || *project.BandwidthLimit < payment.service.config.UsageLimits.Bandwidth.Paid {
			project.BandwidthLimit = new(memory.Size)
			*project.BandwidthLimit = payment.service.config.UsageLimits.Bandwidth.Paid
		}
		if project.SegmentLimit == nil || *project.SegmentLimit < payment.service.config.UsageLimits.Segment.Paid {
			*project.SegmentLimit = payment.service.config.UsageLimits.Segment.Paid
		}
		err = payment.service.store.Projects().Update(ctx, &project)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// MakeCreditCardDefault makes a credit card default payment method.
func (payment Payments) MakeCreditCardDefault(ctx context.Context, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "make credit card default")
	if err != nil {
		return Error.Wrap(err)
	}

	return payment.service.accounts.CreditCards().MakeDefault(ctx, user.ID, cardID)
}

// ProductCharges returns how much money current user will be charged for each project which he owns split by product.
func (payment Payments) ProductCharges(ctx context.Context, since, before time.Time) (_ payments.ProductChargesResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "product charges")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return payment.service.accounts.ProductCharges(ctx, user.ID, since, before)
}

// ShouldApplyMinimumCharge checks if the minimum charge should be applied to the user.
func (payment Payments) ShouldApplyMinimumCharge(ctx context.Context) (bool, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "should apply minimum charge")
	if err != nil {
		return false, ErrUnauthorized.Wrap(err)
	}

	if payment.service.minimumChargeAmount <= 0 {
		return false, nil // no minimum charge configured.
	}

	skip, err := payment.service.accounts.ShouldSkipMinimumCharge(ctx, "", user.ID)
	if err != nil {
		return false, Error.Wrap(err)
	}

	return !skip, nil
}

// GetCardSetupSecret returns a secret to be used by the front end
// to begin card authorization flow.
func (payment Payments) GetCardSetupSecret(ctx context.Context) (secret string, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = payment.service.getUserAndAuditLog(ctx, "start card setup")
	if err != nil {
		return "", ErrUnauthorized.Wrap(err)
	}

	secret, err = payment.service.accounts.CreditCards().GetSetupSecret(ctx)
	if err != nil {
		return "", Error.Wrap(err)
	}

	return secret, nil
}

// AddFunds starts the process of adding funds to the user's account.
func (payment Payments) AddFunds(ctx context.Context, params payments.AddFundsParams) (response *payments.ChargeCardResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "add funds", zap.String("intent", params.Intent.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	if params.Amount < payment.service.config.MinAddFundsAmount {
		return nil, ErrValidation.New("amount is too low")
	}
	if params.Amount > payment.service.config.MaxAddFundsAmount {
		return nil, ErrValidation.New("amount is too high")
	}

	response, err = payment.service.accounts.PaymentIntents().ChargeCard(ctx, payments.ChargeCardRequest{
		CardID: params.CardID,
		CreateIntentParams: payments.CreateIntentParams{
			UserID:   user.ID,
			Amount:   int64(params.Amount),
			Metadata: map[string]string{"user_id": user.ID.String(), params.Intent.String(): "1"},
		},
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return response, nil
}

// CreateIntent creates a payment intent for adding funds to the user's account.
func (payment Payments) CreateIntent(ctx context.Context, amount int, withCustomCard bool) (clientSecret string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "create payment intent")
	if err != nil {
		return "", ErrUnauthorized.Wrap(err)
	}

	if amount < payment.service.config.MinAddFundsAmount {
		return "", ErrValidation.New("amount is too low")
	}
	if amount > payment.service.config.MaxAddFundsAmount {
		return "", ErrValidation.New("amount is too high")
	}

	clientSecret, err = payment.service.accounts.PaymentIntents().Create(ctx, payments.CreateIntentParams{
		UserID:         user.ID,
		Amount:         int64(amount),
		Metadata:       map[string]string{"user_id": user.ID.String(), payments.AddFundsIntent.String(): "1"},
		WithCustomCard: withCustomCard,
	})
	if err != nil {
		return "", Error.Wrap(err)
	}

	return clientSecret, nil
}

// HandleWebhookEvent handles any event from payment provider.
func (payment Payments) HandleWebhookEvent(ctx context.Context, signature string, payload []byte) (err error) {
	defer mon.Task()(&ctx)(&err)

	event, err := payment.service.accounts.WebhookEvents().ParseEvent(ctx, signature, payload)
	if err != nil {
		return Error.Wrap(err)
	}
	if event == nil {
		return nil
	}

	switch event.Type {
	case payments.EventTypePaymentIntentSucceeded:
		if err = payment.handlePaymentIntentSucceeded(ctx, event); err != nil {
			return err
		}
	case payments.EventTypePaymentIntentPaymentFailed:
		payment.service.log.Warn("Payment intent payment failed", zap.String("event_id", event.ID))
	default:
		payment.service.log.Info("Unhandled event type", zap.String("event_type", string(event.Type)), zap.String("event_id", event.ID))
	}

	return nil
}

func (payment Payments) handlePaymentIntentSucceeded(ctx context.Context, event *payments.WebhookEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Unlikely to happen, but just in case.
	if event == nil {
		return Error.New("webhook event is nil")
	}

	metadata, ok := event.Data["metadata"].(map[string]interface{})
	if !ok {
		return Error.New("webhook event metadata missing or invalid")
	}

	_, addFundsFound := metadata[payments.AddFundsIntent.String()]
	if !addFundsFound {
		// We ignore this event if it's not related to adding funds or account upgrade.
		// Most likely it's related to a paid invoice.
		return nil
	}

	userIDStr, ok := metadata["user_id"].(string)
	if !ok {
		return Error.New("user_id missing in webhook event metadata")
	}

	amount, ok := event.Data["amount_received"].(float64)
	if !ok {
		return Error.New("amount_received missing in webhook event data")
	}

	userID, err := uuid.FromString(userIDStr)
	if err != nil {
		return Error.Wrap(err)
	}

	var idempotencyKey string
	if dataID, ok := event.Data["id"].(string); ok {
		idempotencyKey = fmt.Sprintf("%s:%s", dataID, event.Type)
	}

	description := "Credit applied via webhook event: " + event.ID

	_, err = payment.service.accounts.Balances().ApplyCredit(ctx, userID, int64(amount), description, idempotencyKey)
	if err != nil {
		return Error.Wrap(err)
	}

	user, err := payment.service.store.Users().Get(ctx, userID)
	if err != nil {
		payment.service.log.Error("Failed to get user for payment intent succeeded event", zap.String("ID", userID.String()), zap.Error(err))
	} else {
		if user.IsFreeOrMember() {
			// If the user is on a free tier, we upgrade them to paid tier.
			err = payment.upgradeToPaidTier(ctx, user)
			if err != nil {
				payment.service.log.Error("Failed to upgrade user", zap.String("ID", user.ID.String()), zap.Error(err))
			} else {
				payment.service.mailService.SendRenderedAsync(
					ctx,
					[]post.Address{{Address: user.Email}},
					&UpgradeToProEmail{LoginURL: payment.service.config.LoginURL},
				)
			}
		}
	}

	return nil
}

// ListCreditCards returns a list of credit cards for a given payment account.
func (payment Payments) ListCreditCards(ctx context.Context) (_ []payments.CreditCard, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "list credit cards")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = payment.service.accounts.EnsureUserHasCustomer(ctx, user.ID, user.Email, user.SignupPromoCode)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return payment.service.accounts.CreditCards().List(ctx, user.ID)
}

// RemoveCreditCard is used to detach a credit card from payment account.
func (payment Payments) RemoveCreditCard(ctx context.Context, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "remove credit card")
	if err != nil {
		return Error.Wrap(err)
	}

	return payment.service.accounts.CreditCards().Remove(ctx, user.ID, cardID, false)
}

// BillingHistory returns a list of billing history items for payment account.
func (payment Payments) BillingHistory(ctx context.Context) (billingHistory []*BillingHistoryItem, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "get billing history")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	invoices, couponUsages, err := payment.service.accounts.Invoices().ListWithDiscounts(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, invoice := range invoices {
		billingHistory = append(billingHistory, &BillingHistoryItem{
			ID:          invoice.ID,
			Description: invoice.Description,
			Amount:      invoice.Amount,
			Status:      invoice.Status,
			Link:        invoice.Link,
			End:         invoice.End,
			Start:       invoice.Start,
			Type:        Invoice,
		})
	}

	txsInfos, err := payment.service.accounts.StorjTokens().ListTransactionInfos(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, info := range txsInfos {
		billingHistory = append(billingHistory, &BillingHistoryItem{
			ID:          info.ID.String(),
			Description: "STORJ Token Deposit",
			Amount:      info.AmountCents,
			Received:    info.ReceivedCents,
			Status:      info.Status.String(),
			Link:        info.Link,
			Start:       info.CreatedAt,
			End:         info.ExpiresAt,
			Type:        Transaction,
		})
	}

	charges, err := payment.service.accounts.Charges(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, charge := range charges {
		desc := fmt.Sprintf("Payment(%s %s)", charge.CardInfo.Brand, charge.CardInfo.LastFour)

		billingHistory = append(billingHistory, &BillingHistoryItem{
			ID:          charge.ID,
			Description: desc,
			Amount:      charge.Amount,
			Start:       charge.CreatedAt,
			Type:        Charge,
		})
	}

	for _, usage := range couponUsages {
		desc := "Coupon"
		if usage.Coupon.Name != "" {
			desc = usage.Coupon.Name
		}
		if usage.Coupon.PromoCode != "" {
			desc += " (" + usage.Coupon.PromoCode + ")"
		}

		billingHistory = append(billingHistory, &BillingHistoryItem{
			Description: desc,
			Amount:      usage.Amount,
			Start:       usage.PeriodStart,
			End:         usage.PeriodEnd,
			Type:        Coupon,
		})
	}

	bonuses, err := payment.service.accounts.StorjTokens().ListDepositBonuses(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, bonus := range bonuses {
		billingHistory = append(billingHistory,
			&BillingHistoryItem{
				Description: fmt.Sprintf("%d%% Bonus for STORJ Token Deposit", bonus.Percentage),
				Amount:      bonus.AmountCents,
				Status:      "Added to balance",
				Start:       bonus.CreatedAt,
				Type:        DepositBonus,
			},
		)
	}

	sort.SliceStable(billingHistory,
		func(i, j int) bool {
			return billingHistory[i].Start.After(billingHistory[j].Start)
		},
	)

	return billingHistory, nil
}

// InvoiceHistory returns a paged list of invoices for payment account.
func (payment Payments) InvoiceHistory(ctx context.Context, cursor payments.InvoiceCursor) (history *BillingHistoryPage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "get invoice history")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	page, err := payment.service.accounts.Invoices().ListPaged(ctx, user.ID, cursor)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var historyItems []BillingHistoryItem
	for _, invoice := range page.Invoices {
		historyItems = append(historyItems, BillingHistoryItem{
			ID:          invoice.ID,
			Description: invoice.Description,
			Amount:      invoice.Amount,
			Status:      invoice.Status,
			Link:        invoice.Link,
			End:         invoice.End,
			Start:       invoice.Start,
			Type:        Invoice,
		})
	}

	return &BillingHistoryPage{
		Items:    historyItems,
		Next:     page.Next,
		Previous: page.Previous,
	}, nil
}

// checkProjectUsageStatus returns error if for the given project there is some usage for current or previous month.
func (payment Payments) checkProjectUsageStatus(ctx context.Context, project Project) (currentUsage, invoicingIncomplete bool, currentMonthPrice decimal.Decimal, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = payment.service.getUserAndAuditLog(ctx, "project usage status")
	if err != nil {
		return false, false, decimal.Zero, Error.Wrap(err)
	}

	return payment.service.accounts.CheckProjectUsageStatus(ctx, project.ID, project.PublicID)
}

// ApplyCoupon applies a coupon to an account based on couponID.
func (payment Payments) ApplyCoupon(ctx context.Context, couponID string) (coupon *payments.Coupon, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "apply coupon")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	coupon, err = payment.service.accounts.Coupons().ApplyCoupon(ctx, user.ID, couponID)
	if err != nil {
		return coupon, Error.Wrap(err)
	}
	return coupon, nil
}

// ApplyFreeTierCoupon applies the default free tier coupon to an account.
func (payment Payments) ApplyFreeTierCoupon(ctx context.Context) (coupon *payments.Coupon, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := GetUser(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	coupon, err = payment.service.accounts.Coupons().ApplyFreeTierCoupon(ctx, user.ID)
	if err != nil {
		return coupon, Error.Wrap(err)
	}

	return coupon, nil
}

// ApplyCouponCode applies a coupon code to a Stripe customer
// and returns the coupon corresponding to the code.
func (payment Payments) ApplyCouponCode(ctx context.Context, couponCode string) (coupon *payments.Coupon, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "apply coupon code")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	coupon, err = payment.service.accounts.Coupons().ApplyCouponCode(ctx, user.ID, couponCode)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return coupon, nil
}

// GetCoupon returns the coupon applied to the user's account.
func (payment Payments) GetCoupon(ctx context.Context) (coupon *payments.Coupon, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "get coupon")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	coupon, err = payment.service.accounts.Coupons().GetByUserID(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return coupon, nil
}

// AttemptPayOverdueInvoices attempts to pay a user's open, overdue invoices.
func (payment Payments) AttemptPayOverdueInvoices(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "attempt to pay overdue invoices")
	if err != nil {
		return Error.Wrap(err)
	}

	err = payment.service.accounts.Invoices().AttemptPayOverdueInvoices(ctx, user.ID)
	if err != nil {
		payment.service.log.Warn("error attempting to pay overdue invoices for user", zap.String("user id", user.ID.String()), zap.Error(err))
		return Error.Wrap(err)
	}

	return nil
}

// checkRegistrationSecret returns a RegistrationToken if applicable (nil if not), and an error
// if and only if the registration shouldn't proceed.
func (s *Service) checkRegistrationSecret(ctx context.Context, tokenSecret RegistrationSecret) (*RegistrationToken, error) {
	if s.config.OpenRegistrationEnabled && tokenSecret.IsZero() {
		// in this case we're going to let the registration happen without a token
		return nil, nil
	}

	// in all other cases, require a registration token
	registrationToken, err := s.store.RegistrationTokens().GetBySecret(ctx, tokenSecret)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}
	// if a registration token is already associated with an user ID, that means the token is already used
	// we should terminate the account creation process and return an error
	if registrationToken.OwnerID != nil {
		return nil, ErrValidation.New(usedRegTokenErrMsg)
	}

	return registrationToken, nil
}

// VerifyRegistrationCaptcha verifies the registration captcha response.
func (s *Service) VerifyRegistrationCaptcha(ctx context.Context, captchaResp, userIP string) (valid bool, score *float64, err error) {
	defer mon.Task()(&ctx)(&err)
	if s.registrationCaptchaHandler != nil {
		return s.registrationCaptchaHandler.Verify(ctx, captchaResp, userIP)
	}
	return true, nil, nil
}

// ValidateSecurityToken validates a signed security token.
func (s *Service) ValidateSecurityToken(value string) error {
	token, err := consoleauth.FromBase64URLString(value)
	if err != nil {
		return err
	}

	valid, err := s.tokens.ValidateToken(token)
	if err != nil {
		return err
	}
	if !valid {
		return errs.New("Invalid security token")
	}

	return nil
}

// CreateUser gets password hash value and creates new inactive User.
func (s *Service) CreateUser(ctx context.Context, user CreateUser, tokenSecret RegistrationSecret) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)

	mon.Counter("create_user_attempt").Inc(1)

	if err := user.IsValid(user.AllowNoName); err != nil {
		// NOTE: error is already wrapped with an appropriated class.
		return nil, err
	}

	registrationToken, err := s.checkRegistrationSecret(ctx, tokenSecret)
	if err != nil {
		return nil, ErrRegToken.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), s.config.PasswordCost)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// store data
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		userID, err := uuid.New()
		if err != nil {
			return err
		}

		var tenantID *string
		tenantCtx := tenancy.GetContext(ctx)
		if tenantCtx != nil {
			tenantID = &tenantCtx.TenantID
		}

		newUser := &User{
			ID:               userID,
			Email:            user.Email,
			TenantID:         tenantID,
			FullName:         user.FullName,
			ShortName:        user.ShortName,
			PasswordHash:     hash,
			Status:           Inactive,
			IsProfessional:   user.IsProfessional,
			Position:         user.Position,
			CompanyName:      user.CompanyName,
			EmployeeCount:    user.EmployeeCount,
			HaveSalesContact: user.HaveSalesContact,
			SignupPromoCode:  user.SignupPromoCode,
			SignupCaptcha:    user.CaptchaScore,
			ActivationCode:   user.ActivationCode,
			SignupId:         user.SignupId,
			Kind:             user.Kind,
		}

		if user.UserAgent != nil {
			newUser.UserAgent = user.UserAgent
		}

		if registrationToken != nil {
			newUser.ProjectLimit = registrationToken.ProjectLimit
		} else {
			newUser.ProjectLimit = s.config.UsageLimits.Project.Free
		}

		if !user.NoTrialExpiration && s.config.FreeTrialDuration != 0 {
			expiration := s.nowFn().Add(s.config.FreeTrialDuration)
			newUser.TrialExpiration = &expiration
		}

		// TODO: move the project limits into the registration token.
		newUser.ProjectStorageLimit = s.config.UsageLimits.Storage.Free.Int64()
		newUser.ProjectBandwidthLimit = s.config.UsageLimits.Bandwidth.Free.Int64()
		newUser.ProjectSegmentLimit = s.config.UsageLimits.Segment.Free

		u, err = tx.Users().Insert(ctx,
			newUser,
		)
		if err != nil {
			return err
		}

		verified, unverified, err := tx.Users().GetByEmailAndTenantWithUnverified(ctx, user.Email, newUser.TenantID)
		if err != nil {
			return err
		}

		if verified != nil {
			err = tx.Users().Delete(ctx, u.ID)
			if err != nil {
				return err
			}
			mon.Counter("create_user_duplicate_verified").Inc(1)
			return ErrEmailUsed.New(emailUsedErrMsg)
		}

		for _, other := range unverified {
			// We compare IDs because a parallel user creation transaction for the same
			// email could have created a record at the same time as ours.
			if other.CreatedAt.Before(u.CreatedAt) || other.ID.Less(u.ID) {
				err = tx.Users().Delete(ctx, u.ID)
				if err != nil {
					return err
				}
				mon.Counter("create_user_duplicate_unverified").Inc(1)
				return ErrEmailUsed.New(emailUsedErrMsg)
			}
		}

		if registrationToken != nil {
			err = tx.RegistrationTokens().UpdateOwner(ctx, registrationToken.Secret, u.ID)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, Error.Wrap(err)
	}

	s.auditLog(ctx, "create user", nil, user.Email)
	mon.Counter("create_user_success").Inc(1)

	return u, nil
}

// UpdateUserHubspotObjectID updates user's hubspot object ID value.
func (s *Service) UpdateUserHubspotObjectID(ctx context.Context, userID uuid.UUID, objectID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.GetUser(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}

	s.auditLog(ctx, "update user's hubspot object id", &user.ID, user.Email)

	objectIDPtr := &objectID
	return s.store.Users().Update(ctx, userID, UpdateUserRequest{HubspotObjectID: &objectIDPtr})
}

// UpdateUserOnSignup gets new password hash value and updates old inactive User.
func (s *Service) UpdateUserOnSignup(ctx context.Context, inactiveUser *User, requestData CreateUser) (err error) {
	defer mon.Task()(&ctx)(&err)

	// Unlikely, but we should check if the user is still inactive.
	if inactiveUser.Status != Inactive {
		// We return some generic error message to avoid leaking information.
		return Error.New("An error occurred while processing your request. %s", contactSupportErrMsg)
	}

	if err = requestData.IsValid(requestData.AllowNoName); err != nil {
		// NOTE: error is already wrapped with an appropriated class.
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(requestData.Password), s.config.PasswordCost)
	if err != nil {
		return Error.Wrap(err)
	}

	updatedUser := UpdateUserRequest{
		FullName:         &requestData.FullName,
		PasswordHash:     hash,
		IsProfessional:   &requestData.IsProfessional,
		Position:         &requestData.Position,
		CompanyName:      &requestData.CompanyName,
		EmployeeCount:    &requestData.EmployeeCount,
		HaveSalesContact: &requestData.HaveSalesContact,
		ActivationCode:   &requestData.ActivationCode,
		SignupId:         &requestData.SignupId,
		SignupPromoCode:  &requestData.SignupPromoCode,
		Kind:             &requestData.Kind,
	}
	if requestData.ShortName != "" {
		shortNamePtr := &requestData.ShortName
		updatedUser.ShortName = &shortNamePtr
	}
	if requestData.UserAgent != nil {
		updatedUser.UserAgent = requestData.UserAgent
	}

	if requestData.NoTrialExpiration {
		updatedUser.TrialExpiration = new(*time.Time)
	} else if s.config.FreeTrialDuration != 0 {
		expiration := s.nowFn().Add(s.config.FreeTrialDuration)
		expirationPtr := &expiration
		updatedUser.TrialExpiration = &expirationPtr
	}

	err = s.store.Users().Update(ctx, inactiveUser.ID, updatedUser)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// ShouldRequireSsoByUser returns whether SSO should be required of a user.
func (s *Service) ShouldRequireSsoByUser(user *User) bool {
	if !s.ssoEnabled {
		return false
	}
	prov := s.ssoService.GetProviderByEmail(user.Email)
	return user.ExternalID != nil && *user.ExternalID != "" && prov != ""
}

// CreateSsoUser creates a user that has been authenticated by SSO provider.
func (s *Service) CreateSsoUser(ctx context.Context, user CreateSsoUser) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)

	mon.Counter("create_user_attempt").Inc(1)

	if _, err = mail.ParseAddress(user.Email); err != nil {
		// NOTE: error is already wrapped with an appropriated class.
		return nil, ErrUnauthorized.Wrap(err)
	}

	if user.FullName == "" {
		return nil, ErrValidation.New("full name is required")
	}
	if user.ExternalId == "" {
		return nil, ErrValidation.New("external ID is required")
	}

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		userID, err := uuid.New()
		if err != nil {
			return err
		}

		newUser := &User{
			ID:           userID,
			ExternalID:   &user.ExternalId,
			Email:        user.Email,
			FullName:     user.FullName,
			PasswordHash: make([]byte, 0),
		}

		if user.UserAgent != nil {
			newUser.UserAgent = user.UserAgent
		}

		newUser.ProjectLimit = s.config.UsageLimits.Project.Free

		if s.config.FreeTrialDuration != 0 {
			expiration := s.nowFn().Add(s.config.FreeTrialDuration)
			newUser.TrialExpiration = &expiration
		}

		newUser.ProjectStorageLimit = s.config.UsageLimits.Storage.Free.Int64()
		newUser.ProjectBandwidthLimit = s.config.UsageLimits.Bandwidth.Free.Int64()
		newUser.ProjectSegmentLimit = s.config.UsageLimits.Segment.Free

		u, err = tx.Users().Insert(ctx, newUser)
		if err != nil {
			return err
		}

		var tenantID *string
		tenantCtx := tenancy.GetContext(ctx)
		if tenantCtx != nil {
			tenantID = &tenantCtx.TenantID
		}
		_, unverified, err := tx.Users().GetByEmailAndTenantWithUnverified(ctx, user.Email, tenantID)
		if err != nil {
			return err
		}

		for _, other := range unverified {
			// We compare IDs because a parallel user creation transaction for the same
			// email could have created a record at the same time as ours.
			// so we take the first one that was created.
			if other.CreatedAt.Before(u.CreatedAt) || other.ID.Less(u.ID) {
				err = tx.Users().Delete(ctx, u.ID)
				if err != nil {
					return err
				}
				otherUser := other
				u = &otherUser
				break
			}
		}

		active := Active
		request := UpdateUserRequest{Status: &active}
		if u.ExternalID == nil {
			// u is one of the previously created unverified users.
			extID := &user.ExternalId
			request.ExternalID = &extID
		}
		err = tx.Users().Update(ctx, u.ID, request)
		if err != nil {
			return err
		}

		u.Status = Active
		u.ExternalID = &user.ExternalId

		return nil
	})

	if err != nil {
		return nil, Error.Wrap(err)
	}

	s.auditLog(ctx, "create sso user", nil, user.Email)
	mon.Counter("create_user_success").Inc(1)

	return u, nil
}

// UpdateExternalID updates the external (SSO) ID of a user, activating
// them if they're not already.
func (s *Service) UpdateExternalID(ctx context.Context, user *User, externalID string) (err error) {
	defer mon.Task()(&ctx)(&err)

	extID := &externalID
	request := UpdateUserRequest{ExternalID: &extID}
	if user.Status == Inactive {
		active := Active
		request.Status = &active
	}
	err = s.store.Users().Update(ctx, user.ID, request)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetUserForSsoAuth returns a user based on the SSO claims, creating one if necessary.
func (s *Service) GetUserForSsoAuth(ctx context.Context, claims sso.OidcSsoClaims, provider, ip, userAgent string) (user *User, err error) {
	defer mon.Task()(&ctx)(&err)

	externalID := fmt.Sprintf("%s:%s", provider, claims.Sub)
	user, err = s.GetUserByExternalID(ctx, externalID)
	if err != nil {
		if !ErrExternalIdNotFound.Has(err) {
			return nil, err
		}

		user, _, err = s.GetUserByEmailWithUnverified(ctx, claims.Email)
		if err != nil && !ErrEmailNotFound.Has(err) {
			if !ErrEmailNotFound.Has(err) {
				return nil, err
			}
		}
		if user == nil {
			user, err = s.CreateSsoUser(ctx,
				CreateSsoUser{
					FullName:   claims.Name,
					ExternalId: externalID,
					Email:      claims.Email,
					IP:         ip,
					UserAgent:  []byte(userAgent),
				},
			)
			if err != nil {
				return nil, err
			}
		}
	}

	if user.ExternalID == nil || *user.ExternalID != externalID {
		s.log.Info("updating external ID", zap.String("userID", user.ID.String()), zap.String("email", user.Email))
		// associate existing user with this external ID.
		err = s.UpdateExternalID(ctx, user, externalID)
		if err != nil {
			return nil, err
		}
		user.ExternalID = &externalID
	}

	return user, nil
}

// TestSwapCaptchaHandler replaces the existing handler for captchas with
// the one specified for use in testing.
func (s *Service) TestSwapCaptchaHandler(h CaptchaHandler) {
	s.registrationCaptchaHandler = h
	s.loginCaptchaHandler = h
}

// GenerateActivationToken - is a method for generating activation token.
func (s *Service) GenerateActivationToken(ctx context.Context, id uuid.UUID, email string) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.tokens.CreateToken(ctx, id, email)
}

// GeneratePasswordRecoveryToken - is a method for generating password recovery token.
func (s *Service) GeneratePasswordRecoveryToken(ctx context.Context, user *User) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	if s.ShouldRequireSsoByUser(user) {
		s.auditLog(ctx, "sso user attempted 'forgot password' flow", &user.ID, user.Email)
		return "", ErrSsoUserRestricted.New("SSO users cannot reset their password")
	}

	resetPasswordToken, err := s.store.ResetPasswordTokens().GetByOwnerID(ctx, user.ID)
	if err == nil {
		err := s.store.ResetPasswordTokens().Delete(ctx, resetPasswordToken.Secret)
		if err != nil {
			return "", Error.Wrap(err)
		}
	}

	resetPasswordToken, err = s.store.ResetPasswordTokens().Create(ctx, user.ID)
	if err != nil {
		return "", Error.Wrap(err)
	}

	s.auditLog(ctx, "generate password recovery token", &user.ID, "")

	return resetPasswordToken.Secret.String(), nil
}

// SessionTokenRequest contains information needed to create a session token.
type SessionTokenRequest struct {
	UserID          uuid.UUID
	TenantID        *string
	Email           string
	IP              string
	UserAgent       string
	AnonymousID     string
	CustomDuration  *time.Duration
	HubspotObjectID *string
}

// GenerateSessionToken creates a new session and returns the string representation of its token.
func (s *Service) GenerateSessionToken(ctx context.Context, req SessionTokenRequest) (_ *TokenInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	sessionID, err := uuid.New()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	duration := s.config.Session.Duration
	if req.CustomDuration != nil {
		duration = *req.CustomDuration
	} else if s.config.Session.InactivityTimerEnabled {
		settings, err := s.store.Users().GetSettings(ctx, req.UserID)
		if err != nil && !errs.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}
		if settings != nil && settings.SessionDuration != nil {
			duration = *settings.SessionDuration
		} else {
			duration = time.Duration(s.config.Session.InactivityTimerDuration) * time.Second
		}
	}
	expiresAt := time.Now().Add(duration)

	_, err = s.store.WebappSessions().Create(ctx, sessionID, req.UserID, req.IP, req.UserAgent, expiresAt)
	if err != nil {
		return nil, err
	}

	token := consoleauth.Token{Payload: sessionID.Bytes()}

	signature, err := s.tokens.SignToken(token)
	if err != nil {
		return nil, err
	}
	token.Signature = signature

	s.auditLog(ctx, "login", &req.UserID, req.Email)

	s.analytics.TrackSignedIn(req.UserID, req.Email, req.AnonymousID, req.HubspotObjectID, req.TenantID)

	return &TokenInfo{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

// ActivateAccount - is a method for activating user account after registration.
func (s *Service) ActivateAccount(ctx context.Context, activationToken string) (user *User, err error) {
	defer mon.Task()(&ctx)(&err)

	parsedActivationToken, err := consoleauth.FromBase64URLString(activationToken)
	if err != nil {
		return nil, ErrTokenInvalid.Wrap(err)
	}

	valid, err := s.tokens.ValidateToken(parsedActivationToken)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if !valid {
		return nil, ErrTokenInvalid.New("incorrect signature")
	}

	claims, err := consoleauth.FromJSON(parsedActivationToken.Payload)
	if err != nil {
		return nil, ErrTokenInvalid.New("JSON decoder: %w", err)
	}

	if time.Now().After(claims.Expiration) {
		return nil, ErrTokenExpiration.New(activationTokenExpiredErrMsg)
	}

	var tenantID *string
	tenantCtx := tenancy.GetContext(ctx)
	if tenantCtx != nil {
		tenantID = &tenantCtx.TenantID
	}
	_, err = s.store.Users().GetByEmailAndTenant(ctx, claims.Email, tenantID)
	if err == nil {
		return nil, ErrEmailUsed.New(emailUsedErrMsg)
	}

	user, err = s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = s.SetAccountActive(ctx, user)
	if err != nil {
		return nil, err
	}

	user.Status = Active

	return user, nil
}

// SetAccountActive - is a method for setting user account status to Active and sending
// event to hubspot.
func (s *Service) SetAccountActive(ctx context.Context, user *User) (err error) {
	defer mon.Task()(&ctx)(&err)

	if s.config.Captcha.FlagBotsEnabled && user.SignupCaptcha != nil && *user.SignupCaptcha >= s.config.Captcha.ScoreCutoffThreshold {
		minDelay := s.config.Captcha.MinFlagBotDelay
		maxDelay := s.config.Captcha.MaxFlagBotDelay
		rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
		days := rng.Intn(maxDelay-minDelay+1) + minDelay

		err = s.accountFreezeService.DelayedBotFreezeUser(ctx, user.ID, &days)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	activeStatus := Active
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		Status: &activeStatus,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	s.auditLog(ctx, "activate account", &user.ID, user.Email)
	s.analytics.TrackAccountVerified(user.ID, user.Email, user.HubspotObjectID, user.TenantID)

	return nil
}

// SetActivationCodeAndSignupID - generates and updates a new code for user's signup verification.
// It updates the request ID associated with the signup as well.
func (s *Service) SetActivationCodeAndSignupID(ctx context.Context, user User) (_ User, err error) {
	defer mon.Task()(&ctx)(&err)

	if user.Status != Inactive {
		s.auditLog(ctx, "set activation code attempted on active user", &user.ID, user.Email)
		return User{}, ErrActivationCode.New("user already active")
	}

	code, err := generateVerificationCode()
	if err != nil {
		return User{}, Error.Wrap(err)
	}

	requestID := requestid.FromContext(ctx)
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		ActivationCode: &code,
		SignupId:       &requestID,
	})
	if err != nil {
		return User{}, Error.Wrap(err)
	}

	user.SignupId = requestID
	user.ActivationCode = code

	return user, nil
}

// ResetPassword - is a method for resetting user password.
func (s *Service) ResetPassword(ctx context.Context, resetPasswordToken, password string, passcode string, recoveryCode string, t time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	secret, err := ResetPasswordSecretFromBase64(resetPasswordToken)
	if err != nil {
		return ErrRecoveryToken.Wrap(err)
	}
	token, err := s.store.ResetPasswordTokens().GetBySecret(ctx, secret)
	if err != nil {
		return ErrRecoveryToken.Wrap(err)
	}

	user, err := s.store.Users().Get(ctx, *token.OwnerID)
	if err != nil {
		return Error.Wrap(err)
	}

	if user.MFAEnabled {
		now := time.Now()
		if user.LoginLockoutExpiration.After(now) {
			mon.Counter("reset_password_2fa_locked_out").Inc(1)
			s.auditLog(ctx, "reset password: 2fa failed account locked out", &user.ID, user.Email)
			return ErrTooManyAttempts.New(tooManyAttemptsErrMsg)
		}

		handleLockAccount := func() error {
			lockoutDuration, err := s.UpdateUsersFailedLoginState(ctx, user)
			if err != nil {
				return err
			}

			if lockoutDuration > 0 {
				s.mailService.SendRenderedAsync(
					ctx,
					[]post.Address{{Address: user.Email, Name: user.FullName}},
					&LoginLockAccountEmail{
						LockoutDuration: lockoutDuration,
						ActivityType:    MfaAccountLock,
					},
				)
			}

			mon.Counter("reset_password_2fa_failed").Inc(1)
			mon.IntVal("reset_password_2fa_failed_count").Observe(int64(user.FailedLoginCount))

			if user.FailedLoginCount == s.config.LoginAttemptsWithoutPenalty {
				mon.Counter("reset_password_2fa_lockout_initiated").Inc(1)
				s.auditLog(ctx, "reset password: failed reset password 2fa count reached maximum attempts", &user.ID, user.Email)
			}

			if user.FailedLoginCount > s.config.LoginAttemptsWithoutPenalty {
				mon.Counter("reset_password_2fa_lockout_reinitiated").Inc(1)
				s.auditLog(ctx, "reset password: 2fa failed locked account", &user.ID, user.Email)
			}

			return nil
		}

		if recoveryCode != "" {
			found := false
			for _, code := range user.MFARecoveryCodes {
				if code == recoveryCode {
					found = true
					break
				}
			}
			if !found {
				err = handleLockAccount()
				if err != nil {
					return Error.Wrap(err)
				}
				return ErrValidation.Wrap(ErrMFARecoveryCode.New(mfaRecoveryInvalidErrMsg))
			}
		} else if passcode != "" {
			valid, err := ValidateMFAPasscode(passcode, user.MFASecretKey, t)
			if err != nil {
				return ErrValidation.Wrap(ErrMFAPasscode.Wrap(err))
			}
			if !valid {
				err = handleLockAccount()
				if err != nil {
					return Error.Wrap(err)
				}
				return ErrValidation.Wrap(ErrMFAPasscode.New(mfaPasscodeInvalidErrMsg))
			}
		} else {
			return ErrMFAMissing.New(mfaRequiredErrMsg)
		}
	}

	if err := ValidateNewPassword(password); err != nil {
		return ErrValidation.Wrap(err)
	}

	if s.tokens.IsExpired(t, token.CreatedAt) {
		return ErrRecoveryToken.Wrap(ErrTokenExpiration.New(passwordRecoveryTokenIsExpiredErrMsg))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.config.PasswordCost)
	if err != nil {
		return Error.Wrap(err)
	}

	updateRequest := UpdateUserRequest{
		PasswordHash: hash,
	}

	if user.FailedLoginCount != 0 {
		resetLoginLockoutExpirationPtr := &time.Time{}
		updateRequest.LoginLockoutExpiration = &resetLoginLockoutExpirationPtr
		updateRequest.FailedLoginCount = new(int)
	}

	err = s.store.Users().Update(ctx, user.ID, updateRequest)
	if err != nil {
		return Error.Wrap(err)
	}

	s.auditLog(ctx, "password reset", &user.ID, user.Email)

	if err = s.store.ResetPasswordTokens().Delete(ctx, token.Secret); err != nil {
		return Error.Wrap(err)
	}

	_, err = s.store.WebappSessions().DeleteAllByUserID(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// RevokeResetPasswordToken - is a method to revoke reset password token.
func (s *Service) RevokeResetPasswordToken(ctx context.Context, resetPasswordToken string) (err error) {
	defer mon.Task()(&ctx)(&err)

	secret, err := ResetPasswordSecretFromBase64(resetPasswordToken)
	if err != nil {
		return Error.Wrap(err)
	}

	return s.store.ResetPasswordTokens().Delete(ctx, secret)
}

// Token authenticates User by credentials and returns session token.
func (s *Service) Token(ctx context.Context, request AuthUser) (response *TokenInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	mon.Counter("login_attempt").Inc(1)

	verifyCaptcha := func() error {
		if s.config.Captcha.Login.Recaptcha.Enabled || s.config.Captcha.Login.Hcaptcha.Enabled {
			valid, _, err := s.loginCaptchaHandler.Verify(ctx, request.CaptchaResponse, request.IP)
			if err != nil {
				mon.Counter("login_user_captcha_error").Inc(1)
				return ErrCaptcha.Wrap(err)
			}
			if !valid {
				mon.Counter("login_user_captcha_unsuccessful").Inc(1)
				return ErrCaptcha.New("captcha validation unsuccessful")
			}
		}
		return nil
	}

	captchaSkipped := true
	if request.MFARecoveryCode == "" && request.MFAPasscode == "" {
		// verify captcha on first login attempt.
		// we only want to verify captcha if the user is not verifying MFA.
		err = verifyCaptcha()
		if err != nil {
			return nil, err
		}
		captchaSkipped = false
	}

	var tenantID *string
	tenantCtx := tenancy.GetContext(ctx)
	if tenantCtx != nil {
		tenantID = &tenantCtx.TenantID
	}
	user, nonActiveUsers, err := s.store.Users().GetByEmailAndTenantWithUnverified(ctx, request.Email, tenantID)
	if user == nil {
		shouldProceed := false
		for _, usr := range nonActiveUsers {
			if usr.Status == PendingBotVerification || usr.Status == LegalHold {
				shouldProceed = true
				botAccount := usr
				user = &botAccount
				break
			}
		}

		if !shouldProceed {
			if len(nonActiveUsers) > 0 {
				mon.Counter("login_email_unverified").Inc(1)
				s.auditLog(ctx, "login: failed email unverified", nil, request.Email)
			} else {
				mon.Counter("login_email_invalid").Inc(1)
				s.auditLog(ctx, "login: failed invalid email", nil, request.Email)
			}
			return nil, ErrLoginCredentials.New(credentialsErrMsg)
		}
	}

	if user.LoginLockoutExpiration.After(time.Now()) {
		mon.Counter("login_locked_out").Inc(1)
		s.auditLog(ctx, "login: failed account locked out", &user.ID, request.Email)
		return nil, ErrLoginCredentials.New(credentialsErrMsg)
	}

	if s.ShouldRequireSsoByUser(user) {
		s.auditLog(ctx, "login: attempted sso bypass", &user.ID, request.Email)
		return nil, ErrSsoUserRestricted.New(credentialsErrMsg)
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(request.Password))
	if err != nil {
		err = s.handleLogInLockAccount(ctx, user)
		if err != nil {
			return nil, err
		}
		mon.Counter("login_invalid_password").Inc(1)
		s.auditLog(ctx, "login: failed password invalid", &user.ID, user.Email)
		return nil, ErrLoginCredentials.New(credentialsErrMsg)
	}

	if user.Status == PendingBotVerification || user.Status == LegalHold {
		return nil, ErrLoginRestricted.New("")
	}

	if user.MFAEnabled {
		err = s.logInVerifyMFA(ctx, user, request)
		if err != nil {
			return nil, err
		}
	} else if captchaSkipped {
		// captcha was skipped because mfa fields were provided in the request,
		// but user does not have mfa enabled, so we still need to verify captcha.
		err = verifyCaptcha()
		if err != nil {
			return nil, err
		}
	}

	if user.FailedLoginCount != 0 {
		err = s.ResetAccountLock(ctx, user)
		if err != nil {
			return nil, err
		}
	}

	var customDurationPtr *time.Duration
	if request.RememberForOneWeek {
		weekDuration := 7 * 24 * time.Hour
		customDurationPtr = &weekDuration
	}
	response, err = s.GenerateSessionToken(ctx, SessionTokenRequest{
		UserID:          user.ID,
		TenantID:        user.TenantID,
		Email:           user.Email,
		IP:              request.IP,
		UserAgent:       request.UserAgent,
		AnonymousID:     request.AnonymousID,
		CustomDuration:  customDurationPtr,
		HubspotObjectID: user.HubspotObjectID,
	})
	if err != nil {
		return nil, err
	}

	mon.Counter("login_success").Inc(1)

	return response, nil
}

func (s *Service) handleLogInLockAccount(ctx context.Context, user *User) error {
	lockoutDuration, err := s.UpdateUsersFailedLoginState(ctx, user)
	if err != nil {
		return err
	}
	if lockoutDuration > 0 {
		address := s.satelliteAddress
		if !strings.HasSuffix(address, "/") {
			address += "/"
		}

		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email, Name: user.FullName}},
			&LoginLockAccountEmail{
				LockoutDuration:   lockoutDuration,
				ResetPasswordLink: address + "forgot-password",
				ActivityType:      LoginAccountLock,
			},
		)
	}

	mon.Counter("login_failed").Inc(1)
	mon.IntVal("login_user_failed_count").Observe(int64(user.FailedLoginCount))

	if user.FailedLoginCount == s.config.LoginAttemptsWithoutPenalty {
		mon.Counter("login_lockout_initiated").Inc(1)
		s.auditLog(ctx, "login: failed login count reached maximum attempts", &user.ID, user.Email)
	}

	if user.FailedLoginCount > s.config.LoginAttemptsWithoutPenalty {
		mon.Counter("login_lockout_reinitiated").Inc(1)
		s.auditLog(ctx, "login: failed locked account", &user.ID, user.Email)
	}

	return nil
}

func (s *Service) logInVerifyMFA(ctx context.Context, user *User, request AuthUser) (err error) {
	defer mon.Task()(&ctx)(&err)

	if request.MFARecoveryCode != "" && request.MFAPasscode != "" {
		mon.Counter("login_mfa_conflict").Inc(1)
		s.auditLog(ctx, "login: failed mfa conflict", &user.ID, user.Email)
		return ErrMFAConflict.New(mfaConflictErrMsg)
	}

	if request.MFARecoveryCode != "" {
		found := false
		codeIndex := -1
		for i, code := range user.MFARecoveryCodes {
			if code == request.MFARecoveryCode {
				found = true
				codeIndex = i
				break
			}
		}
		if !found {
			err = s.handleLogInLockAccount(ctx, user)
			if err != nil {
				return err
			}
			mon.Counter("login_mfa_recovery_failure").Inc(1)
			s.auditLog(ctx, "login: failed mfa recovery", &user.ID, user.Email)
			return ErrMFARecoveryCode.New(mfaRecoveryInvalidErrMsg)
		}

		mon.Counter("login_mfa_recovery_success").Inc(1)

		user.MFARecoveryCodes = append(user.MFARecoveryCodes[:codeIndex], user.MFARecoveryCodes[codeIndex+1:]...)

		err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
			MFARecoveryCodes: &user.MFARecoveryCodes,
		})
		if err != nil {
			return err
		}
	} else if request.MFAPasscode != "" {
		valid, err := ValidateMFAPasscode(request.MFAPasscode, user.MFASecretKey, time.Now())
		if err != nil {
			newErr := s.handleLogInLockAccount(ctx, user)
			if newErr != nil {
				return newErr
			}

			return ErrMFAPasscode.Wrap(err)
		}
		if !valid {
			err = s.handleLogInLockAccount(ctx, user)
			if err != nil {
				return err
			}
			mon.Counter("login_mfa_passcode_failure").Inc(1)
			s.auditLog(ctx, "login: failed mfa passcode invalid", &user.ID, user.Email)
			return ErrMFAPasscode.New(mfaPasscodeInvalidErrMsg)
		}
		mon.Counter("login_mfa_passcode_success").Inc(1)
	} else {
		mon.Counter("login_mfa_missing").Inc(1)
		s.auditLog(ctx, "login: failed mfa missing", &user.ID, user.Email)
		return ErrMFAMissing.New(mfaRequiredErrMsg)
	}

	if user.FailedLoginCount != 0 {
		err = s.ResetAccountLock(ctx, user)
		if err != nil {
			return err
		}
	}

	return nil
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

// TokenByAPIKey authenticates User by API Key and returns session token.
func (s *Service) TokenByAPIKey(ctx context.Context, userAgent string, ip string, apiKey string) (response *TokenInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	userID, _, err := s.GetUserAndExpirationFromKey(ctx, apiKey)
	if err != nil {
		return nil, ErrUnauthorized.New(apiKeyCredentialsErrMsg)
	}

	user, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return nil, Error.New(failedToRetrieveUserErrMsg)
	}

	response, err = s.GenerateSessionToken(ctx, SessionTokenRequest{
		UserID:          user.ID,
		TenantID:        user.TenantID,
		Email:           user.Email,
		IP:              ip,
		UserAgent:       userAgent,
		AnonymousID:     "",
		CustomDuration:  nil,
		HubspotObjectID: user.HubspotObjectID,
	})
	if err != nil {
		return nil, Error.New(generateSessionTokenErrMsg)
	}

	return response, nil
}

// UpdateUsersFailedLoginState updates User's failed login state.
func (s *Service) UpdateUsersFailedLoginState(ctx context.Context, user *User) (lockoutDuration time.Duration, err error) {
	defer mon.Task()(&ctx)(&err)

	var failedLoginPenalty *float64
	if user.FailedLoginCount >= s.config.LoginAttemptsWithoutPenalty-1 {
		lockoutDuration = time.Duration(math.Pow(s.config.FailedLoginPenalty, float64(user.FailedLoginCount-1))) * time.Minute
		failedLoginPenalty = &s.config.FailedLoginPenalty
	}

	return lockoutDuration, s.store.Users().UpdateFailedLoginCountAndExpiration(ctx, failedLoginPenalty, user.ID, s.nowFn())
}

// GetLoginAttemptsWithoutPenalty returns LoginAttemptsWithoutPenalty config value.
func (s *Service) GetLoginAttemptsWithoutPenalty() int {
	return s.config.LoginAttemptsWithoutPenalty
}

// ResetAccountLock resets a user's failed login count and lockout duration.
func (s *Service) ResetAccountLock(ctx context.Context, user *User) (err error) {
	defer mon.Task()(&ctx)(&err)

	user.FailedLoginCount = 0
	loginLockoutExpirationPtr := &time.Time{}
	return s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		FailedLoginCount:       &user.FailedLoginCount,
		LoginLockoutExpiration: &loginLockoutExpirationPtr,
	})
}

// GetUser returns User by id.
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.store.Users().Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return user, nil
}

// GenGetUser returns ResponseUser by request context for generated api.
func (s *Service) GenGetUser(ctx context.Context) (*ResponseUser, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get user")
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	respUser := &ResponseUser{
		ID:                   user.ID,
		FullName:             user.FullName,
		ShortName:            user.ShortName,
		Email:                user.Email,
		UserAgent:            user.UserAgent,
		ProjectLimit:         user.ProjectLimit,
		IsProfessional:       user.IsProfessional,
		Position:             user.Position,
		CompanyName:          user.CompanyName,
		EmployeeCount:        user.EmployeeCount,
		HaveSalesContact:     user.HaveSalesContact,
		PaidTier:             user.IsPaid(),
		MFAEnabled:           user.MFAEnabled,
		MFARecoveryCodeCount: len(user.MFARecoveryCodes),
	}

	return respUser, api.HTTPError{}
}

// GetUserID returns the User ID from the session.
func (s *Service) GetUserID(ctx context.Context) (id uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get user ID")
	if err != nil {
		return uuid.UUID{}, Error.Wrap(err)
	}
	return user.ID, nil
}

// GetUserByEmailWithUnverified returns Users by email.
func (s *Service) GetUserByEmailWithUnverified(ctx context.Context, email string) (verified *User, unverified []User, err error) {
	defer mon.Task()(&ctx)(&err)

	var tenantID *string
	tenantCtx := tenancy.GetContext(ctx)
	if tenantCtx != nil {
		tenantID = &tenantCtx.TenantID
	}

	verified, unverified, err = s.store.Users().GetByEmailAndTenantWithUnverified(ctx, email, tenantID)
	if err != nil {
		return verified, unverified, err
	}

	if verified == nil && len(unverified) == 0 {
		err = ErrEmailNotFound.New(emailNotFoundErrMsg)
	}

	return verified, unverified, err
}

// GetUserByExternalID returns a user with specified external ID.
func (s *Service) GetUserByExternalID(ctx context.Context, externalID string) (user *User, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err = s.store.Users().GetByExternalID(ctx, externalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExternalIdNotFound.New("user not found")
		}
		return nil, Error.Wrap(err)
	}

	return user, nil
}

// GetUserHasVarPartner returns whether the user in context is associated with a VAR partner.
func (s *Service) GetUserHasVarPartner(ctx context.Context) (has bool, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "get user has VAR partner")
	if err != nil {
		return false, Error.Wrap(err)
	}

	if _, has = s.varPartners[string(user.UserAgent)]; has {
		return has, nil
	}
	return false, nil
}

// accountAction stands for account action to be tracked.
type accountAction = string

// AccountActionStep stands for each explicit change email flow step.
type AccountActionStep = int

const (
	// DeleteAccountInit is the initial step of account deletion where we check the user
	// has met all account deletion requirements before then verifying password etc.
	DeleteAccountInit AccountActionStep = 0
	// DeleteProjectInit is the initial step of project deletion where we check the user
	// has met all project deletion requirements before then verifying password etc.
	DeleteProjectInit AccountActionStep = 0
	// VerifyAccountPasswordStep stands for the first step of the change email/account delete flow
	// where user has to provide an account password.
	VerifyAccountPasswordStep AccountActionStep = 1
	// VerifyAccountMfaStep stands for the second step of the change email/account delete flow
	// where user has to provide a 2fa passcode.
	VerifyAccountMfaStep AccountActionStep = 2
	// VerifyAccountEmailStep stands for the third step of the change email/account delete flow
	// where user has to provide an OTP code sent to their current email address.
	VerifyAccountEmailStep AccountActionStep = 3
	// DeleteAccountStep stands for the last step of the delete account flow
	// where user has to approve the intention to delete account.
	DeleteAccountStep AccountActionStep = 4
	// DeleteProjectStep stands for the last step of the delete project flow
	// where user has to approve the intention to delete project.
	DeleteProjectStep AccountActionStep = 4
	// ChangeAccountEmailStep stands for the fourth step of the change email flow
	// where user has to provide a new email address.
	ChangeAccountEmailStep AccountActionStep = 4
	// VerifyNewAccountEmailStep stands for the fifth step of the change email flow
	// where user has to provide an OTP code sent to their new email address.
	VerifyNewAccountEmailStep AccountActionStep = 5

	changeEmailAction   accountAction = "change_email"
	deleteAccountAction accountAction = "delete_account"
	deleteProjectAction accountAction = "delete_project"

	// SkipObjectLockEnabledBuckets is a flag to skip checking for object lock enabled buckets
	// during project or account deletion.
	SkipObjectLockEnabledBuckets = "skip-object-lock-enabled-buckets"
)

// DeleteAccount handles self-serve account delete actions.
func (s *Service) DeleteAccount(ctx context.Context, step AccountActionStep, data string) (resp *DeleteAccountResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.SelfServeAccountDeleteEnabled {
		return nil, ErrForbidden.New("this feature is disabled")
	}

	user, err := s.getUserAndAuditLog(ctx, "delete account")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if user.Status == LegalHold {
		return nil, ErrForbidden.New("account can't be deleted")
	}

	if user.ExternalID != nil && *user.ExternalID != "" {
		return nil, ErrForbidden.New("sso users must contact support to delete their accounts")
	}

	if user.LoginLockoutExpiration.After(s.nowFn()) {
		mon.Counter("delete_account_locked_out").Inc(1)
		s.auditLog(ctx, "delete account: failed account locked out", &user.ID, user.Email)
		return nil, ErrUnauthorized.New("please try again later")
	}

	resp = &DeleteAccountResponse{}
	deletionRestricted := false
	var projects []Project

	if !s.config.AbbreviatedDeleteAccountEnabled || (step == DeleteAccountInit && data != SkipObjectLockEnabledBuckets) {
		projects, err = s.store.Projects().GetOwnActive(ctx, user.ID)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		resp.OwnedProjects = len(projects)
	}

	// check project deletion restrictions
	for _, p := range projects {
		if step == DeleteAccountInit && s.config.AbbreviatedDeleteAccountEnabled && data != SkipObjectLockEnabledBuckets {
			// check for buckets with Object Lock enabled
			for _, p := range projects {
				count, err := s.buckets.CountObjectLockBuckets(ctx, p.ID)
				if err != nil {
					return nil, err
				}
				resp.LockEnabledBuckets += count
			}
			if resp.LockEnabledBuckets > 0 {
				return resp, nil
			}
		}

		if !s.config.AbbreviatedDeleteAccountEnabled {
			buckets, err := s.buckets.CountBuckets(ctx, p.ID)
			if err != nil {
				return nil, err
			}
			if buckets > 0 {
				deletionRestricted = true
				resp.Buckets += buckets
			}

			// ignore object browser api key because we hide it from the user, so they can't delete it.
			// project row deletion cascades to api keys, so it's okay.
			keys, err := s.store.APIKeys().GetPagedByProjectID(ctx, p.ID, APIKeyCursor{Limit: 1, Page: 1}, s.config.ObjectBrowserKeyNamePrefix)
			if err != nil {
				return nil, err
			}
			if keys.TotalCount > 0 {
				deletionRestricted = true
				resp.ApiKeys += int(keys.TotalCount)
			}
		}
	}

	if user.IsPaid() {
		if len(projects) == 0 {
			projects, err = s.store.Projects().GetOwnActive(ctx, user.ID)
			if err != nil {
				return nil, Error.Wrap(err)
			}
		}
		for _, p := range projects {
			currentUsage, invoicingIncomplete, _, err := s.Payments().checkProjectUsageStatus(ctx, p)
			if err != nil && !payments.ErrUnbilledUsage.Has(err) {
				return nil, err
			}

			if currentUsage {
				deletionRestricted = true
				resp.CurrentUsage = true
			}
			if invoicingIncomplete {
				deletionRestricted = true
				resp.InvoicingIncomplete = true
			}
		}
	}

	// check for unpaid invoices.
	invoices, err := s.accounts.Invoices().List(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, invoice := range invoices {
		if invoice.Status == payments.InvoiceStatusOpen || invoice.Status == payments.InvoiceStatusDraft {
			deletionRestricted = true
			resp.UnpaidInvoices++
			resp.AmountOwed += invoice.Amount
		}
	}

	// check for pending invoice items.
	hasItems, err := s.accounts.Invoices().CheckPendingItems(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	if hasItems {
		deletionRestricted = true
		resp.InvoicingIncomplete = true
	}

	if deletionRestricted {
		return resp, nil
	}

	switch step {
	case DeleteAccountInit:
		return nil, nil
	case VerifyAccountPasswordStep:
		return nil, s.handlePasswordStep(ctx, user, data, deleteAccountAction)
	case VerifyAccountMfaStep:
		return nil, s.handleMfaStep(ctx, user, data, deleteAccountAction)
	case VerifyAccountEmailStep:
		return nil, s.handleVerifyCurrentEmailStep(ctx, user, data, deleteAccountAction)
	case DeleteAccountStep:
		return nil, s.handleDeleteAccountStep(ctx, user)
	default:
		return nil, ErrValidation.New("step value is out of range")
	}
}

// ChangeEmail handles change user's email actions.
func (s *Service) ChangeEmail(ctx context.Context, step AccountActionStep, data string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.EmailChangeFlowEnabled {
		return ErrForbidden.New("this feature is disabled")
	}

	user, err := s.getUserAndAuditLog(ctx, "change email")
	if err != nil {
		return Error.Wrap(err)
	}

	if user.LoginLockoutExpiration.After(s.nowFn()) {
		mon.Counter("change_email_locked_out").Inc(1)
		s.auditLog(ctx, "change email: failed account locked out", &user.ID, user.Email)
		return ErrUnauthorized.New("please try again later")
	}

	if user.ExternalID != nil && *user.ExternalID != "" {
		s.auditLog(ctx, "change email: attempted by sso user", &user.ID, user.Email)
		return ErrForbidden.New("sso users cannot change email")
	}

	switch step {
	case VerifyAccountPasswordStep:
		err = s.handlePasswordStep(ctx, user, data, changeEmailAction)
		if err != nil {
			return err
		}

		return nil
	case VerifyAccountMfaStep:
		err = s.handleMfaStep(ctx, user, data, changeEmailAction)
		if err != nil {
			return err
		}

		return nil
	case VerifyAccountEmailStep:
		err = s.handleVerifyCurrentEmailStep(ctx, user, data, changeEmailAction)
		if err != nil {
			return err
		}

		return nil
	case ChangeAccountEmailStep:
		err = s.handleNewEmailStep(ctx, user, data)
		if err != nil {
			return err
		}

		return nil
	case VerifyNewAccountEmailStep:
		err = s.handleVerifyNewStep(ctx, user, data)
		if err != nil {
			return err
		}

		return nil
	default:
		return ErrValidation.New("step value is out of range")
	}
}

func (s *Service) handlePasswordStep(ctx context.Context, user *User, data string, action accountAction) (err error) {
	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(data))
	if err != nil {
		err = s.handleLockAccount(ctx, user, VerifyAccountPasswordStep, action)
		if err != nil {
			return err
		}

		return ErrValidation.New("password is incorrect")
	}

	var verificationCode string
	if !user.MFAEnabled {
		verificationCode, err = generateVerificationCode()
		if err != nil {
			return Error.Wrap(err)
		}
	}

	err = s.updateStep(ctx, user.ID, VerifyAccountPasswordStep, verificationCode, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	if !user.MFAEnabled {
		var emailAction string
		switch action {
		case changeEmailAction:
			emailAction = "an account email address change"
		case deleteAccountAction:
			emailAction = "an account deletion"
		case deleteProjectAction:
			emailAction = "a project deletion"
		default:
			return errs.New("invalid account action: %s", action)
		}

		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email, Name: user.FullName}},
			&EmailAddressVerificationEmail{
				VerificationCode: verificationCode,
				Action:           emailAction,
			},
		)
	}

	return nil
}

func (s *Service) handleMfaStep(ctx context.Context, user *User, data string, action accountAction) (err error) {
	if !user.MFAEnabled {
		return nil
	}

	if user.EmailChangeVerificationStep < VerifyAccountPasswordStep {
		err = s.handleLockAccount(ctx, user, VerifyAccountMfaStep, action)
		if err != nil {
			return err
		}

		return ErrValidation.New(accountActionWrongStepOrderErrMsg)
	}

	valid, err := ValidateMFAPasscode(data, user.MFASecretKey, s.nowFn())
	if err != nil {
		err = s.handleLockAccount(ctx, user, VerifyAccountMfaStep, action)
		if err != nil {
			return err
		}

		return ErrMFAPasscode.Wrap(err)
	}
	if !valid {
		err = s.handleLockAccount(ctx, user, VerifyAccountMfaStep, action)
		if err != nil {
			return err
		}
		mon.Counter("change_email_2fa_passcode_failure").Inc(1)
		s.auditLog(ctx, "change email: failed 2fa passcode invalid", &user.ID, user.Email)
		return ErrMFAPasscode.New(mfaPasscodeInvalidErrMsg)
	}

	verificationCode, err := generateVerificationCode()
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.updateStep(ctx, user.ID, VerifyAccountMfaStep, verificationCode, nil)
	if err != nil {
		return Error.Wrap(err)
	}

	var emailAction string
	switch action {
	case changeEmailAction:
		emailAction = "an account email address change"
	case deleteAccountAction:
		emailAction = "an account deletion"
	case deleteProjectAction:
		emailAction = "a project deletion"
	default:
		return errs.New("invalid account action: %s", action)
	}

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: user.FullName}},
		&EmailAddressVerificationEmail{
			VerificationCode: verificationCode,
			Action:           emailAction,
		},
	)

	return nil
}

func (s *Service) handleVerifyCurrentEmailStep(ctx context.Context, user *User, data string, action accountAction) (err error) {
	previousStep := VerifyAccountPasswordStep
	if user.MFAEnabled {
		previousStep = VerifyAccountMfaStep
	}

	if user.EmailChangeVerificationStep < previousStep {
		err = s.handleLockAccount(ctx, user, VerifyAccountEmailStep, action)
		if err != nil {
			return err
		}

		return ErrValidation.New(accountActionWrongStepOrderErrMsg)
	}

	if user.ActivationCode != data {
		err = s.handleLockAccount(ctx, user, VerifyAccountEmailStep, action)
		if err != nil {
			return err
		}

		return ErrValidation.New("verification code is incorrect")
	}

	err = s.updateStep(ctx, user.ID, VerifyAccountEmailStep, "", nil)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (s *Service) handleDeleteProjectStep(ctx context.Context, user *User, projectID, publicProjectID uuid.UUID, deleteProjectInfo *DeleteProjectInfo) (err error) {
	if user.EmailChangeVerificationStep < VerifyAccountEmailStep {
		err = s.handleLockAccount(ctx, user, DeleteProjectStep, deleteProjectAction)
		if err != nil {
			return err
		}
		return ErrValidation.New(accountActionWrongStepOrderErrMsg)
	}

	if s.config.AbbreviatedDeleteProjectEnabled {
		err = s.store.Projects().UpdateStatus(ctx, projectID, ProjectPendingDeletion)
		if err != nil {
			return err
		}

		currentPriceStr := "0"
		if deleteProjectInfo != nil {
			currentPriceStr = deleteProjectInfo.CurrentMonthPrice.String()
		}

		s.log.Info("project marked for deletion successfully by user",
			zap.String("project_id", publicProjectID.String()),
			zap.String("user_id", user.ID.String()),
			zap.String("user_email", user.Email),
			zap.String("current_usage_price", currentPriceStr),
		)
		s.analytics.TrackProjectDeleted(user.ID, user.Email, publicProjectID, currentPriceStr, user.HubspotObjectID, user.TenantID)

		// We need to reset the step value to prevent the possibility of bypassing steps
		// in subsequent delete project requests.
		return s.store.Users().Update(ctx, user.ID, UpdateUserRequest{EmailChangeVerificationStep: new(int)})
	}

	err = s.store.Domains().DeleteAllByProjectID(ctx, projectID)
	if err != nil {
		s.log.Error("failed to delete all domains for project",
			zap.String("project_id", projectID.String()),
			zap.Error(err),
		)
	}

	err = s.entitlementsService.Projects().DeleteByPublicID(ctx, publicProjectID)
	if err != nil {
		s.log.Error("failed to delete project entitlements",
			zap.String("project_public_id", publicProjectID.String()),
			zap.Error(err),
		)
	}

	// We update status to disabled instead of deleting the project
	// to not lose the historical project/user usage data.
	err = s.store.Projects().UpdateStatus(ctx, projectID, ProjectDisabled)
	if err != nil {
		return err
	}

	currentPriceStr := "0"
	if deleteProjectInfo != nil {
		currentPriceStr = deleteProjectInfo.CurrentMonthPrice.String()
	}

	s.log.Info("project deleted successfully by user",
		zap.String("project_id", publicProjectID.String()),
		zap.String("user_id", user.ID.String()),
		zap.String("user_email", user.Email),
		zap.String("current_usage_price", currentPriceStr),
	)
	s.analytics.TrackProjectDeleted(user.ID, user.Email, publicProjectID, currentPriceStr, user.HubspotObjectID, user.TenantID)

	// We need to reset the step value to prevent the possibility of bypassing steps
	// in subsequent delete project requests.
	return s.store.Users().Update(ctx, user.ID, UpdateUserRequest{EmailChangeVerificationStep: new(int)})
}

func (s *Service) handleDeleteAccountStep(ctx context.Context, user *User) (err error) {
	if user.EmailChangeVerificationStep < VerifyAccountEmailStep {
		err = s.handleLockAccount(ctx, user, DeleteAccountStep, deleteAccountAction)
		if err != nil {
			return err
		}

		return ErrValidation.New(accountActionWrongStepOrderErrMsg)
	}

	status := Deleted
	if s.config.AbbreviatedDeleteAccountEnabled {
		status = PendingDeletion
		err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
			FullName:  new(string),
			ShortName: new(*string),
			Status:    &status,
			// Self-serve account deletion isn't allowed for SSO users, but we keep this here as a precaution.
			ExternalID:                  new(*string),
			EmailChangeVerificationStep: new(int),
		})
		if err != nil {
			return Error.Wrap(err)
		}

		s.log.Info("account marked for deletion successfully by user",
			zap.String("user_id", user.ID.String()),
			zap.String("user_email", user.Email),
		)
		s.analytics.TrackDeleteUser(user.ID, user.Email, user.HubspotObjectID, user.TenantID)

		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email, Name: user.FullName}},
			&AccountDeletionSuccessEmail{},
		)

		return nil
	}

	projects, err := s.store.Projects().GetOwnActive(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	var errsList errs.Group
	for _, p := range projects {
		// We delete all API keys associated with the project as a precaution, in case any still exist.
		err = s.store.APIKeys().DeleteAllByProjectID(ctx, p.ID)
		if err != nil {
			errsList.Add(err)
		}

		err = s.store.Domains().DeleteAllByProjectID(ctx, p.ID)
		if err != nil {
			s.log.Error("failed to delete all domains for project",
				zap.String("project_id", p.ID.String()),
				zap.Error(err),
			)
		}

		err = s.entitlementsService.Projects().DeleteByPublicID(ctx, p.PublicID)
		if err != nil {
			s.log.Error("failed to delete project entitlements",
				zap.String("project_public_id", p.PublicID.String()),
				zap.Error(err),
			)
		}

		// We update status to disabled instead of deleting the project
		// to not lose the historical project/user usage data.
		err = s.store.Projects().UpdateStatus(ctx, p.ID, ProjectDisabled)
		if err != nil {
			errsList.Add(err)
		}
	}
	if errsList.Err() != nil {
		return Error.Wrap(errsList.Err())
	}

	err = s.accounts.CreditCards().RemoveAll(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = s.store.WebappSessions().DeleteAllByUserID(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	deactivatedEmail := fmt.Sprintf("deactivated+%s@storj.io", user.ID.String())
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		FullName:  new(string),
		ShortName: new(*string),
		Email:     &deactivatedEmail,
		Status:    &status,
		// Self-serve account deletion isn't allowed for SSO users, but we keep this here as a precaution.
		ExternalID:                  new(*string),
		EmailChangeVerificationStep: new(int),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	s.log.Info("account deleted successfully by user",
		zap.String("user_id", user.ID.String()),
		zap.String("user_email", user.Email),
	)
	s.analytics.TrackDeleteUser(user.ID, user.Email, user.HubspotObjectID, user.TenantID)

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: user.FullName}},
		&AccountDeletionSuccessEmail{},
	)

	return nil
}

func (s *Service) handleNewEmailStep(ctx context.Context, user *User, data string) (err error) {
	if user.EmailChangeVerificationStep == ChangeAccountEmailStep && user.NewUnverifiedEmail != nil {
		return ErrConflict.New("a new unverified email is already set. Please verify it or restart the flow")
	}

	if user.EmailChangeVerificationStep < VerifyAccountEmailStep {
		err = s.handleLockAccount(ctx, user, ChangeAccountEmailStep, changeEmailAction)
		if err != nil {
			return err
		}

		return ErrValidation.New(accountActionWrongStepOrderErrMsg)
	}

	isValidEmail := utils.ValidateEmail(data)
	if !isValidEmail {
		return ErrValidation.New("invalid email")
	}

	verified, unverified, err := s.store.Users().GetByEmailAndTenantWithUnverified(ctx, data, user.TenantID)
	if err != nil {
		return Error.Wrap(err)
	}

	if verified != nil || len(unverified) > 0 {
		// we throw validation error just not to compromise existing user emails.
		return ErrValidation.New("invalid email")
	}

	verificationCode, err := generateVerificationCode()
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.updateStep(ctx, user.ID, ChangeAccountEmailStep, verificationCode, &data)
	if err != nil {
		return Error.Wrap(err)
	}

	emailAction := "account email address change"

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: data, Name: user.FullName}},
		&EmailAddressVerificationEmail{
			VerificationCode: verificationCode,
			Action:           emailAction,
		},
	)

	return nil
}

func (s *Service) handleVerifyNewStep(ctx context.Context, user *User, data string) (err error) {
	if user.EmailChangeVerificationStep < ChangeAccountEmailStep {
		err = s.handleLockAccount(ctx, user, VerifyNewAccountEmailStep, changeEmailAction)
		if err != nil {
			return err
		}

		return ErrValidation.New(accountActionWrongStepOrderErrMsg)
	}

	if user.ActivationCode != data {
		err = s.handleLockAccount(ctx, user, VerifyNewAccountEmailStep, changeEmailAction)
		if err != nil {
			return err
		}

		return ErrValidation.New("verification code is incorrect")
	}

	// unlikely to happen but still.
	if user.NewUnverifiedEmail == nil {
		return Error.New("new email is not set")
	}

	loginLockoutExpirationPtr := &time.Time{}
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		Email:                       user.NewUnverifiedEmail,
		EmailChangeVerificationStep: new(int),
		FailedLoginCount:            new(int),
		LoginLockoutExpiration:      &loginLockoutExpirationPtr,
		ActivationCode:              new(string),
		NewUnverifiedEmail:          new(*string),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: *user.NewUnverifiedEmail, Name: user.FullName}},
		&ChangeEmailSuccessEmail{},
	)

	if s.config.BillingFeaturesEnabled {
		err = s.Payments().ChangeEmail(ctx, user.ID, *user.NewUnverifiedEmail)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	s.analytics.ChangeContactEmail(user.ID, user.Email, *user.NewUnverifiedEmail)

	return nil
}

func (s *Service) handleLockAccount(ctx context.Context, user *User, step AccountActionStep, action accountAction) error {
	lockoutDuration, err := s.UpdateUsersFailedLoginState(ctx, user)
	if err != nil {
		return err
	}

	var activityType string
	switch action {
	case changeEmailAction:
		activityType = ChangeEmailLock
	case deleteProjectAction:
		activityType = DeleteProjectLock
	case deleteAccountAction:
		activityType = DeleteAccountLock
	default:
		return Error.New("invalid action: %s", action)
	}

	if lockoutDuration > 0 {
		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: user.Email, Name: user.FullName}},
			&LoginLockAccountEmail{
				LockoutDuration: lockoutDuration,
				ActivityType:    activityType,
			},
		)
	}

	switch step {
	case VerifyAccountPasswordStep:
		action += "_password"
	case VerifyAccountMfaStep:
		action += "_2fa"
	case VerifyAccountEmailStep:
		action += "_verify_current_email"
	case VerifyNewAccountEmailStep:
		action += "_verify_new_email"
	}

	mon.Counter(action + "_failed").Inc(1)
	mon.IntVal(action + "_failed_count").Observe(int64(user.FailedLoginCount))

	if user.FailedLoginCount == s.config.LoginAttemptsWithoutPenalty {
		mon.Counter(action + "_lockout_initiated").Inc(1)
		s.auditLog(ctx, fmt.Sprintf("account action: failed %s count reached maximum attempts", action), &user.ID, user.Email)
	}

	if user.FailedLoginCount > s.config.LoginAttemptsWithoutPenalty {
		mon.Counter(action + "_lockout_reinitiated").Inc(1)
		s.auditLog(ctx, fmt.Sprintf("account action: %s failed locked account", action), &user.ID, user.Email)
	}

	return nil
}

func (s *Service) updateStep(ctx context.Context, userID uuid.UUID, step AccountActionStep, verificationCode string, newUnverifiedEmail *string) error {
	loginLockoutExpirationPtr := &time.Time{}

	return s.store.Users().Update(ctx, userID, UpdateUserRequest{
		EmailChangeVerificationStep: &step,
		FailedLoginCount:            new(int),
		LoginLockoutExpiration:      &loginLockoutExpirationPtr,
		ActivationCode:              &verificationCode,
		NewUnverifiedEmail:          &newUnverifiedEmail,
	})
}

func generateVerificationCode() (string, error) {
	randNum, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", err
	}
	randNum = randNum.Add(randNum, big.NewInt(100000))

	return randNum.String(), nil
}

// UpdateAccount updates User.
func (s *Service) UpdateAccount(ctx context.Context, fullName string, shortName string) (err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "update account")
	if err != nil {
		return Error.Wrap(err)
	}

	// validate fullName
	err = ValidateFullName(fullName)
	if err != nil {
		return ErrValidation.Wrap(err)
	}

	err = s.ValidateFreeFormFieldLengths(&fullName, &shortName)
	if err != nil {
		return err
	}

	user.FullName = fullName
	user.ShortName = shortName
	shortNamePtr := &user.ShortName
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		FullName:  &user.FullName,
		ShortName: &shortNamePtr,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// SetupAccount completes User's information.
func (s *Service) SetupAccount(ctx context.Context, requestData SetUpAccountRequest) (err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "setup account")
	if err != nil {
		return Error.Wrap(err)
	}

	fullName, err := s.getValidatedFullName(&requestData)
	if err != nil {
		return ErrValidation.Wrap(err)
	}

	companyName, err := s.getValidatedCompanyName(&requestData)
	if err != nil {
		return ErrValidation.Wrap(err)
	}

	err = s.ValidateFreeFormFieldLengths(
		requestData.StorageUseCase, requestData.OtherUseCase,
		requestData.Position, requestData.FunctionalArea,
	)
	if err != nil {
		return err
	}

	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		FullName:         &fullName,
		IsProfessional:   &requestData.IsProfessional,
		HaveSalesContact: &requestData.HaveSalesContact,
		Position:         requestData.Position,
		CompanyName:      companyName,
		EmployeeCount:    requestData.EmployeeCount,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	onboardingFields := analytics.TrackOnboardingInfoFields{
		ID:              user.ID,
		TenantID:        user.TenantID,
		HubspotObjectID: user.HubspotObjectID,
		FullName:        fullName,
		Email:           user.Email,
	}

	if requestData.StorageUseCase != nil {
		onboardingFields.StorageUseCase = *requestData.StorageUseCase
		if requestData.OtherUseCase != nil {
			onboardingFields.OtherUseCase = *requestData.OtherUseCase
		}
	}

	if requestData.IsProfessional {
		onboardingFields.Type = analytics.Professional
		onboardingFields.HaveSalesContact = requestData.HaveSalesContact
		onboardingFields.InterestedInPartnering = requestData.InterestedInPartnering
		if companyName != nil {
			onboardingFields.CompanyName = *companyName
		}
		if requestData.EmployeeCount != nil {
			onboardingFields.EmployeeCount = *requestData.EmployeeCount
		}
		if requestData.StorageNeeds != nil {
			onboardingFields.StorageNeeds = *requestData.StorageNeeds
		}
		if requestData.Position != nil {
			onboardingFields.JobTitle = *requestData.Position
		}
		if requestData.FunctionalArea != nil {
			onboardingFields.FunctionalArea = *requestData.FunctionalArea
		}
	} else {
		onboardingFields.Type = analytics.Personal
	}
	s.analytics.TrackUserOnboardingInfo(onboardingFields)

	return nil
}

func (s *Service) getValidatedFullName(requestData *SetUpAccountRequest) (name string, err error) {
	if requestData.IsProfessional {
		if requestData.FirstName == nil {
			return "", errs.New("First name wasn't provided")
		}

		if len(*requestData.FirstName) == 0 || len(*requestData.FirstName) > s.config.MaxNameCharacters {
			return "", errs.New("First name length must be more then 0 and less then or equal to %d", s.config.MaxNameCharacters)
		}

		name = *requestData.FirstName

		if requestData.LastName != nil {
			if len(*requestData.LastName) > s.config.MaxNameCharacters {
				return "", errs.New("Last name length must be less then or equal to %d", s.config.MaxNameCharacters)
			}

			name += " " + *requestData.LastName
		}
	} else {
		if requestData.FullName == nil {
			return "", errs.New("Full name wasn't provided")
		}

		if len(*requestData.FullName) == 0 || len(*requestData.FullName) > s.config.MaxNameCharacters {
			return "", errs.New("Full name length must be more then 0 and less then or equal to %d", s.config.MaxNameCharacters)
		}

		name = *requestData.FullName
	}

	return name, nil
}

func (s *Service) getValidatedCompanyName(requestData *SetUpAccountRequest) (name *string, err error) {
	if requestData.IsProfessional {
		if requestData.CompanyName == nil {
			return nil, errs.New("Company name wasn't provided")
		}

		if len(*requestData.CompanyName) == 0 || len(*requestData.CompanyName) > s.config.MaxNameCharacters {
			return nil, errs.New("Company name length must be more then 0 and less then or equal to %d", s.config.MaxNameCharacters)
		}

		name = requestData.CompanyName
	}

	return name, nil
}

// ChangePassword updates password for a given user.
func (s *Service) ChangePassword(ctx context.Context, pass, newPass string, sessionID *uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "change password")
	if err != nil {
		return Error.Wrap(err)
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(pass))
	if err != nil {
		return ErrChangePassword.New(changePasswordErrMsg)
	}

	if err := ValidateNewPassword(newPass); err != nil {
		return ErrValidation.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), s.config.PasswordCost)
	if err != nil {
		return Error.Wrap(err)
	}

	user.PasswordHash = hash
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		PasswordHash: hash,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	address := s.satelliteAddress
	if !strings.HasSuffix(address, "/") {
		address += "/"
	}

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&PasswordChangedEmail{
			ResetPasswordLink: address + "forgot-password",
		},
	)

	resetPasswordToken, err := s.store.ResetPasswordTokens().GetByOwnerID(ctx, user.ID)
	if err == nil {
		err := s.store.ResetPasswordTokens().Delete(ctx, resetPasswordToken.Secret)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	if sessionID != nil {
		err = s.DeleteAllSessionsByUserIDExcept(ctx, user.ID, *sessionID)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// GetProject is a method for querying project by internal or public ID.
func (s *Service) GetProject(ctx context.Context, projectID uuid.UUID) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "get project", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	p = isMember.project

	return
}

// GetProjectNoAuth is a method for querying project by ID or public ID.
// This is for internal use only as it ignores whether a user is authorized to perform this action.
// If authorization checking is required, use GetProject.
func (s *Service) GetProjectNoAuth(ctx context.Context, projectID uuid.UUID) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	p, err = s.store.Projects().GetByPublicID(ctx, projectID)
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			p, err = s.store.Projects().Get(ctx, projectID)
			if err != nil {
				return nil, Error.Wrap(err)
			}
		} else {
			return nil, Error.Wrap(err)
		}
	}

	return p, nil
}

// GetSalt is a method for querying project salt by id.
// id may be project.ID or project.PublicID.
func (s *Service) GetSalt(ctx context.Context, projectID uuid.UUID) (salt []byte, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "get project salt", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	return s.store.Projects().GetSalt(ctx, isMember.project.ID)
}

// JoinProjectNoAuth adds a user to a project with a specified role.
func (s *Service) JoinProjectNoAuth(ctx context.Context, projectID uuid.UUID, user *User, role ProjectMemberRole) {
	// should not happen in practice, but just in case.
	if user == nil {
		return
	}

	_, err := s.store.ProjectMembers().Insert(ctx, user.ID, projectID, role)
	if err != nil {
		s.log.Warn("error adding user to project",
			zap.Error(err),
			zap.String("email", user.Email),
			zap.String("projectID", projectID.String()),
		)
		return
	}

	err = s.store.ProjectInvitations().Delete(ctx, projectID, user.Email)
	if err != nil {
		s.log.Warn("error deleting project invitation",
			zap.Error(err),
			zap.String("email", user.Email),
			zap.String("projectID", projectID.String()),
		)
	}
}

// EmissionImpactResponse represents emission impact response to be returned to client.
type EmissionImpactResponse struct {
	StorjImpact       float64 `json:"storjImpact"`
	HyperscalerImpact float64 `json:"hyperscalerImpact"`
	SavedTrees        int64   `json:"savedTrees"`
}

// GetEmissionImpact is a method for querying project emission impact by id.
func (s *Service) GetEmissionImpact(ctx context.Context, projectID uuid.UUID) (*EmissionImpactResponse, error) {
	var err error
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "get project emission impact", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	storageUsed, err := s.projectUsage.GetProjectStorageTotals(ctx, isMember.project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	now := s.nowFn()
	period := now.Sub(isMember.project.CreatedAt)
	dataInTB := memory.Size(storageUsed).TB()

	impact, err := s.emission.CalculateImpact(&emission.CalculationInput{
		AmountOfDataInTB: dataInTB,
		Duration:         period,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	savedValue := impact.EstimatedKgCO2eHyperscaler - impact.EstimatedKgCO2eStorj
	if savedValue < 0 {
		savedValue = 0
	}

	savedTrees := s.emission.CalculateSavedTrees(savedValue)

	return &EmissionImpactResponse{
		StorjImpact:       impact.EstimatedKgCO2eStorj,
		HyperscalerImpact: impact.EstimatedKgCO2eHyperscaler,
		SavedTrees:        savedTrees,
	}, nil
}

// GetProjectConfig is a method for querying project config.
func (s *Service) GetProjectConfig(ctx context.Context, projectID uuid.UUID) (*ProjectConfig, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get project config", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	project := isMember.project

	salt, err := s.store.Projects().GetSalt(ctx, project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	ownerKind, err := s.store.Users().GetUserKind(ctx, project.OwnerID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	membersCount, err := s.store.ProjectMembers().GetTotalCountByProjectID(ctx, project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var passphrase []byte
	var hasManagedPassphrase bool
	if project.PassphraseEnc != nil {
		hasManagedPassphrase = true
	}
	if project.PassphraseEnc != nil && s.kmsService != nil {
		if project.PassphraseEncKeyID == nil {
			s.analytics.TrackManagedEncryptionError(user.ID, user.Email, project.ID, "nil key ID for project in DB", user.HubspotObjectID, user.TenantID)
			return nil, Error.New("Failed to retrieve passphrase")
		}
		passphrase, err = s.kmsService.DecryptPassphrase(ctx, *project.PassphraseEncKeyID, project.PassphraseEnc)
		if err != nil {
			s.log.Error("failed to decrypt passphrase", zap.Error(err))
			s.analytics.TrackManagedEncryptionError(user.ID, user.Email, project.ID, err.Error(), user.HubspotObjectID, user.TenantID)
			return nil, Error.New("Failed to retrieve passphrase")
		}
	}

	if len(passphrase) == 0 && hasManagedPassphrase {
		// the UI handles this condition on its own, so we track an analytics event, but continue to send a valid response to the client.
		s.analytics.TrackManagedEncryptionError(user.ID, user.Email, project.ID, "kms service not enabled on satellite", user.HubspotObjectID, user.TenantID)
	}

	pathEncryptionEnabled := project.PathEncryption == nil || *project.PathEncryption

	placementDetails, err := s.getPlacementDetails(ctx, project)
	if err != nil {
		return nil, err
	}

	var computeAuthToken string
	if s.entitlementsConfig.Enabled && s.config.ComputeUiEnabled && isMember.membership.Role == RoleAdmin {
		features, err := s.entitlementsService.Projects().GetByPublicID(ctx, project.PublicID)
		if err != nil {
			s.log.Error("failed to get project entitlements", zap.Error(err))
		} else if features.ComputeAccessToken != nil {
			computeAuthToken = string(features.ComputeAccessToken)
		}
	}

	return &ProjectConfig{
		HasManagedPassphrase: hasManagedPassphrase,
		EncryptPath:          pathEncryptionEnabled,
		Passphrase:           string(passphrase),
		IsOwnerPaidTier:      ownerKind == PaidUser,
		HasPaidPrivileges:    ownerKind == PaidUser || ownerKind == NFRUser,
		Role:                 isMember.membership.Role,
		Salt:                 base64.StdEncoding.EncodeToString(salt),
		MembersCount:         membersCount,
		AvailablePlacements:  placementDetails,
		ComputeAuthToken:     computeAuthToken,
	}, nil
}

// GetObjectLockUIEnabled returns whether object lock is enabled.
func (s *Service) GetObjectLockUIEnabled() bool {
	return s.objectLockAndVersioningConfig.UseBucketLevelObjectVersioning && s.objectLockAndVersioningConfig.ObjectLockEnabled
}

// GetUsersProjects is a method for querying all projects.
func (s *Service) GetUsersProjects(ctx context.Context) (ps []Project, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get users projects")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	ps, err = s.store.Projects().GetActiveByUserID(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for i, project := range ps {
		project.StorageUsed, project.BandwidthUsed, err = s.getStorageAndBandwidthUse(ctx, project.ID)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		if s.entitlementsConfig.Enabled && s.legacyPlacements != nil {
			if ent, err := s.entitlementsService.Projects().GetByPublicID(ctx, project.PublicID); err == nil && ent.NewBucketPlacements != nil {
				project.IsClassic = slices.Equal(ent.NewBucketPlacements, s.legacyPlacements)
			}
		}

		ps[i] = project
	}

	return ps, nil
}

// GetMinimalProject returns a ProjectInfo copy of a project.
func (s *Service) GetMinimalProject(project *Project) ProjectInfo {
	info := ProjectInfo{
		ID:                   project.PublicID,
		Name:                 project.Name,
		OwnerID:              project.OwnerID,
		Description:          project.Description,
		MemberCount:          project.MemberCount,
		CreatedAt:            project.CreatedAt,
		StorageUsed:          project.StorageUsed,
		BandwidthUsed:        project.BandwidthUsed,
		Versioning:           project.DefaultVersioning,
		Placement:            project.DefaultPlacement,
		HasManagedPassphrase: project.PassphraseEnc != nil,
		IsClassic:            project.IsClassic,
	}

	if edgeURLs, ok := s.config.PlacementEdgeURLOverrides.Get(project.DefaultPlacement); ok {
		info.EdgeURLOverrides = &edgeURLs
	}

	return info
}

// GenGetUsersProjects is a method for querying all projects for generated api.
func (s *Service) GenGetUsersProjects(ctx context.Context) (ps []Project, httpErr api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get users projects")
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	ps, err = s.store.Projects().GetActiveByUserID(ctx, user.ID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	return
}

// JoinCunoFSBeta is a method for tracking user joined cunoFS beta.
func (s *Service) JoinCunoFSBeta(ctx context.Context, data analytics.TrackJoinCunoFSBetaFields) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "join cunoFS beta")
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	if user.Status == PendingBotVerification {
		return ErrBotUser.New(contactSupportErrMsg)
	}

	settings, err := s.store.Users().GetSettings(ctx, user.ID)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return Error.Wrap(err)
		}
	}

	var noticeDismissal NoticeDismissal
	betaJoined := false
	if settings != nil {
		betaJoined = settings.NoticeDismissal.CunoFSBetaJoined
		noticeDismissal = settings.NoticeDismissal
	}
	if betaJoined {
		return ErrConflict.New("user already joined cunoFS beta")
	}

	data.Email = user.Email

	s.analytics.JoinCunoFSBeta(data)

	noticeDismissal.CunoFSBetaJoined = true
	err = s.store.Users().UpsertSettings(ctx, user.ID, UpsertUserSettingsRequest{
		NoticeDismissal: &noticeDismissal,
	})
	if err != nil {
		return errs.Combine(Error.New("Your submission was successfully received, but something else went wrong"), err)
	}

	return nil
}

// SendUserFeedback is a method for tracking user feedback submission.
func (s *Service) SendUserFeedback(ctx context.Context, data analytics.UserFeedbackFormData) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.UserFeedbackEnabled {
		return ErrForbidden.New("User feedback feature is disabled")
	}

	user, err := s.getUserAndAuditLog(ctx, "send user feedback")
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}
	if user.Status == PendingBotVerification {
		return ErrBotUser.New(contactSupportErrMsg)
	}

	props := map[string]string{
		"feedback_type": data.Type,
		"message":       data.Message,
		"allow_contact": strconv.FormatBool(data.AllowContact),
	}
	s.analytics.TrackEvent(analytics.EventUserFeedbackSubmitted, user.ID, user.Email, props, user.HubspotObjectID, user.TenantID)

	return nil
}

// JoinPlacementWaitlist is a method for adding user to a placement waitlist.
func (s *Service) JoinPlacementWaitlist(ctx context.Context, data analytics.TrackJoinPlacementWaitlistFields) (err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.Placement.SelfServeEnabled {
		return Error.New("Self-serve placement is disabled")
	}

	user, err := s.getUserAndAuditLog(ctx, "join placement waitlist")
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	if user.Status == PendingBotVerification {
		return ErrBotUser.New(contactSupportErrMsg)
	}

	settings, err := s.store.Users().GetSettings(ctx, user.ID)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return Error.Wrap(err)
		}
	}

	var noticeDismissal NoticeDismissal
	waitlistJoined := false
	if settings != nil {
		waitlistsJoined := settings.NoticeDismissal.PlacementWaitlistsJoined
		for _, constraint := range waitlistsJoined {
			if constraint == data.Placement {
				waitlistJoined = true
				break
			}
		}
		noticeDismissal = settings.NoticeDismissal
	}
	if waitlistJoined {
		return ErrConflict.New("user already joined waitlist")
	}

	data.Email = user.Email
	placement, ok := s.config.Placement.SelfServeDetails.Get(data.Placement)
	if !ok {
		return ErrPlacementNotFound.New("")
	}

	data.WaitlistURL = placement.WaitlistURL
	s.analytics.JoinPlacementWaitlist(data)

	noticeDismissal.PlacementWaitlistsJoined = append(noticeDismissal.PlacementWaitlistsJoined, storj.PlacementConstraint(placement.ID))
	err = s.store.Users().UpsertSettings(ctx, user.ID, UpsertUserSettingsRequest{
		NoticeDismissal: &noticeDismissal,
	})
	if err != nil {
		return errs.Combine(Error.New("Your submission was successfully received, but something else went wrong"), err)
	}

	return nil
}

// RequestObjectMountConsultation is a method for tracking user requested object mount consultation.
func (s *Service) RequestObjectMountConsultation(ctx context.Context, data analytics.TrackObjectMountConsultationFields) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "request object mount consultation")
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	if user.Status == PendingBotVerification {
		return ErrBotUser.New(contactSupportErrMsg)
	}

	settings, err := s.store.Users().GetSettings(ctx, user.ID)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return Error.Wrap(err)
		}
	}

	var noticeDismissal NoticeDismissal
	requested := false
	if settings != nil {
		requested = settings.NoticeDismissal.ObjectMountConsultationRequested
		noticeDismissal = settings.NoticeDismissal
	}
	if requested {
		return ErrConflict.New("user already requested object mount consultation")
	}

	data.Email = user.Email

	s.analytics.RequestObjectMountConsultation(data)

	noticeDismissal.ObjectMountConsultationRequested = true
	err = s.store.Users().UpsertSettings(ctx, user.ID, UpsertUserSettingsRequest{
		NoticeDismissal: &noticeDismissal,
	})
	if err != nil {
		return errs.Combine(Error.New("Your submission was successfully received, but something else went wrong"), err)
	}

	return nil
}

// CreateProject is a method for creating new project.
func (s *Service) CreateProject(ctx context.Context, projectInfo UpsertProjectInfo) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "create project")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if user.Status == PendingBotVerification {
		return nil, ErrBotUser.New(contactSupportErrMsg)
	}

	err = ValidateNameAndDescription(projectInfo.Name, projectInfo.Description)
	if err != nil {
		return nil, ErrValidation.Wrap(err)
	}

	currentProjectCount, err := s.checkProjectLimit(ctx, user.ID)
	if err != nil {
		s.analytics.TrackProjectLimitError(user.ID, user.Email, user.HubspotObjectID, user.TenantID)
		return nil, ErrProjLimit.Wrap(err)
	}

	passesNameCheck, err := s.checkProjectName(ctx, projectInfo, user.ID)
	if err != nil || !passesNameCheck {
		return nil, ErrProjName.Wrap(err)
	}

	newProjectLimits, err := s.getUserProjectLimits(ctx, user.ID)
	if err != nil {
		return nil, ErrProjLimit.Wrap(err)
	}

	var (
		projectID            uuid.UUID
		satManagedPassphrase bool
	)
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		storageLimit := memory.Size(newProjectLimits.Storage)
		bandwidthLimit := memory.Size(newProjectLimits.Bandwidth)

		newProject := &Project{
			Description:      projectInfo.Description,
			Name:             projectInfo.Name,
			OwnerID:          user.ID,
			UserAgent:        user.UserAgent,
			StorageLimit:     &storageLimit,
			BandwidthLimit:   &bandwidthLimit,
			SegmentLimit:     &newProjectLimits.Segment,
			DefaultPlacement: user.DefaultPlacement,
		}

		if user.DefaultPlacement == storj.DefaultPlacement && s.entitlementsConfig.Enabled && len(s.config.Placement.AllowedPlacementIdsForNewProjects) > 0 {
			newProject.DefaultPlacement = s.config.Placement.AllowedPlacementIdsForNewProjects[0]
		}

		if s.config.SatelliteManagedEncryptionEnabled && projectInfo.ManagePassphrase && s.kmsService != nil {
			encPassphrase, keyID, err := s.kmsService.GenerateEncryptedPassphrase(ctx)
			if err != nil {
				return Error.Wrap(err)
			}
			newProject.PassphraseEnc = encPassphrase
			newProject.PassphraseEncKeyID = &keyID
			newProject.PathEncryption = &s.config.ManagedEncryption.PathEncryptionEnabled

			satManagedPassphrase = true
		} else if projectInfo.ManagePassphrase {
			return ErrSatelliteManagedEncryption
		}

		p, err = tx.Projects().Insert(ctx, newProject)
		if err != nil {
			return Error.Wrap(err)
		}

		limit, err := tx.Users().GetProjectLimit(ctx, user.ID)
		if err != nil {
			return err
		}

		projects, err := tx.Projects().GetOwnActive(ctx, user.ID)
		if err != nil {
			return err
		}

		// We check again for project name duplication and whether the project limit
		// has been exceeded in case a parallel project creation transaction created
		// a project at the same time as this one.
		var numBefore int
		for _, other := range projects {
			if other.CreatedAt.Before(p.CreatedAt) || (other.CreatedAt.Equal(p.CreatedAt) && other.ID.Less(p.ID)) {
				if other.Name == p.Name {
					return errs.Combine(ErrProjName.New(projNameErrMsg), tx.Projects().Delete(ctx, p.ID))
				}
				numBefore++
			}
		}
		if numBefore >= limit {
			s.analytics.TrackProjectLimitError(user.ID, user.Email, user.HubspotObjectID, user.TenantID)
			return errs.Combine(ErrProjLimit.New(projLimitErrMsg), tx.Projects().Delete(ctx, p.ID))
		}

		_, err = tx.ProjectMembers().Insert(ctx, user.ID, p.ID, RoleAdmin)
		if err != nil {
			return Error.Wrap(err)
		}

		if s.entitlementsConfig.Enabled {
			// We have to use a direct DB call here because we are in a transaction.
			partnerMap, defaultMap := s.accounts.GetPlacementProductMappings(string(newProject.UserAgent))
			mapping := entitlements.PlacementProductMappings{}
			for placement, productID := range partnerMap {
				mapping[storj.PlacementConstraint(placement)] = productID
			}
			for placement, productID := range defaultMap {
				if _, exists := mapping[storj.PlacementConstraint(placement)]; !exists {
					mapping[storj.PlacementConstraint(placement)] = productID
				}
			}
			feats := entitlements.ProjectFeatures{
				NewBucketPlacements:      s.config.Placement.AllowedPlacementIdsForNewProjects,
				PlacementProductMappings: mapping,
			}
			if user.DefaultPlacement != storj.DefaultPlacement {
				feats.NewBucketPlacements = []storj.PlacementConstraint{user.DefaultPlacement}
			}
			featBytes, err := json.Marshal(feats)
			if err != nil {
				return Error.Wrap(err)
			}

			_, err = tx.Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
				Scope:     entitlements.ConvertPublicIDToProjectScope(p.PublicID),
				Features:  featBytes,
				UpdatedAt: s.nowFn(),
			})
			if err != nil {
				return Error.Wrap(err)
			}
		}

		projectID = p.ID

		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	s.analytics.TrackProjectCreated(user.ID, user.Email, projectID, currentProjectCount+1, satManagedPassphrase, user.HubspotObjectID, user.TenantID)

	return p, nil
}

// GenCreateProject is a method for creating new project for generated api.
func (s *Service) GenCreateProject(ctx context.Context, projectInfo UpsertProjectInfo) (p *Project, httpError api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	p, err = s.CreateProject(ctx, projectInfo)
	if err != nil {
		status := http.StatusInternalServerError
		if _, ctxErr := GetUser(ctx); ctxErr != nil {
			status = http.StatusUnauthorized
		}
		return nil, api.HTTPError{
			Status: status,
			Err:    err,
		}
	}

	return p, httpError
}

// DeleteProject is a method for deleting project by id.
func (s *Service) DeleteProject(ctx context.Context, projectID uuid.UUID, step AccountActionStep, data string) (info *DeleteProjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.DeleteProjectEnabled {
		return nil, ErrForbidden.New("this feature is disabled")
	}

	user, err := s.getUserAndAuditLog(ctx, "delete project", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if user.ExternalID != nil && *user.ExternalID != "" {
		return nil, ErrForbidden.New("sso users must ask support to delete projects")
	}

	_, p, err := s.isProjectOwner(ctx, user.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectID = p.ID

	if user.LoginLockoutExpiration.After(s.nowFn()) {
		mon.Counter("delete_project_locked_out").Inc(1)
		s.auditLog(ctx, "delete project: failed account locked out", &user.ID, user.Email)
		return nil, ErrUnauthorized.New("please try again later")
	}

	info, err = s.checkProjectCanBeDeleted(ctx, user, p, step, data)
	if err != nil {
		return info, Error.Wrap(err)
	}

	switch step {
	case DeleteProjectInit:
		return nil, nil
	case VerifyAccountPasswordStep:
		return nil, s.handlePasswordStep(ctx, user, data, deleteProjectAction)
	case VerifyAccountMfaStep:
		return nil, s.handleMfaStep(ctx, user, data, deleteProjectAction)
	case VerifyAccountEmailStep:
		return nil, s.handleVerifyCurrentEmailStep(ctx, user, data, deleteProjectAction)
	case DeleteProjectStep:
		return nil, s.handleDeleteProjectStep(ctx, user, projectID, p.PublicID, info)
	default:
		return nil, ErrValidation.New("step value is out of range")
	}
}

// GenDeleteProject is a method for deleting project by id for generated API.
func (s *Service) GenDeleteProject(ctx context.Context, projectID uuid.UUID) (httpError api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "delete project", zap.String("projectID", projectID.String()))
	if err != nil {
		return api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	_, p, err := s.isProjectOwner(ctx, user.ID, projectID)
	if err != nil {
		status := http.StatusInternalServerError
		if ErrUnauthorized.Has(err) {
			status = http.StatusUnauthorized
		}
		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	projectID = p.ID

	info, err := s.checkProjectCanBeDeleted(ctx, user, p, DeleteProjectInit, "")
	if err != nil {
		return api.HTTPError{
			Status: http.StatusConflict,
			Err:    Error.Wrap(err),
		}
	}

	// We update status to disabled instead of deleting the project
	// to not lose the historical project/user usage data.
	err = s.store.Projects().UpdateStatus(ctx, projectID, ProjectDisabled)
	if err != nil {
		return api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	currentPriceStr := "0"
	if info != nil {
		currentPriceStr = info.CurrentMonthPrice.String()
	}

	s.log.Info("project deleted successfully",
		zap.String("project_id", p.PublicID.String()),
		zap.String("user_id", user.ID.String()),
		zap.String("user_email", user.Email),
		zap.String("current_usage_price", currentPriceStr),
	)
	s.analytics.TrackProjectDeleted(user.ID, user.Email, p.PublicID, currentPriceStr, user.HubspotObjectID, user.TenantID)

	return httpError
}

// UpdateProject is a method for updating project name and description by id.
// projectID here may be project.PublicID or project.ID.
func (s *Service) UpdateProject(ctx context.Context, projectID uuid.UUID, updatedProject UpsertProjectInfo) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "update project name and description", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, project, err := s.isProjectOwner(ctx, user.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = ValidateNameAndDescription(updatedProject.Name, updatedProject.Description)
	if err != nil {
		return nil, ErrValidation.Wrap(err)
	}

	if updatedProject.Name != project.Name {
		passesNameCheck, err := s.checkProjectName(ctx, updatedProject, user.ID)
		if err != nil || !passesNameCheck {
			return nil, ErrProjName.Wrap(err)
		}
	}
	project.Name = updatedProject.Name
	project.Description = updatedProject.Description

	if user.HasPaidPrivileges() {
		err = s.validateLimits(ctx, project, UpdateLimitsInfo{
			StorageLimit:   updatedProject.StorageLimit,
			BandwidthLimit: updatedProject.BandwidthLimit,
		}, false)
		if err != nil {
			return nil, err
		}
		if updatedProject.StorageLimit != nil {
			project.UserSpecifiedStorageLimit = updatedProject.StorageLimit
		}
		if updatedProject.BandwidthLimit != nil {
			project.UserSpecifiedBandwidthLimit = updatedProject.BandwidthLimit
		}
	}

	err = s.store.Projects().Update(ctx, project)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return project, nil
}

// UpdateUserSpecifiedLimits is a method for updating project user specified limits.
func (s *Service) UpdateUserSpecifiedLimits(ctx context.Context, projectID uuid.UUID, updatedLimits UpdateLimitsInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "update project limits", zap.String("projectID", projectID.String()))
	if err != nil {
		return Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}
	project := isMember.project

	if isMember.membership.Role != RoleAdmin && project.OwnerID != user.ID {
		return ErrUnauthorized.New("Only project owner or admin may update project limits")
	}

	kind := user.Kind
	if project.OwnerID != user.ID {
		kind, err = s.store.Users().GetUserKind(ctx, project.OwnerID)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	if kind == FreeUser {
		return ErrNotPaidTier.New("Only Pro users may update project limits")
	}

	updates := make([]Limit, 0)
	err = s.validateLimits(ctx, project, updatedLimits, true)
	if err != nil {
		return err
	}

	if updatedLimits.StorageLimit != nil {
		limit := new(int64)
		*limit = updatedLimits.StorageLimit.Int64()
		if *limit == 0 {
			limit = nil
		}
		updates = append(updates, Limit{
			Kind:  UserSetStorageLimit,
			Value: limit,
		})
	}

	if updatedLimits.BandwidthLimit != nil {
		limit := new(int64)
		*limit = updatedLimits.BandwidthLimit.Int64()
		if *limit == 0 {
			limit = nil
		}
		updates = append(updates, Limit{
			Kind:  UserSetBandwidthLimit,
			Value: limit,
		})
	}

	err = s.store.Projects().UpdateLimitsGeneric(ctx, project.ID, updates)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

func (s *Service) validateLimits(ctx context.Context, project *Project, updatedLimits UpdateLimitsInfo, allowZero bool) (err error) {
	defer mon.Task()(&ctx)(&err)

	if updatedLimits.StorageLimit != nil {
		if !allowZero && *updatedLimits.StorageLimit <= 0 {
			return ErrInvalidProjectLimit.New("Project limits must be greater than 0")
		}
		if project.StorageLimit != nil && *project.StorageLimit == 0 {
			return Error.New("current storage limit for project is set to 0 (updating disabled)")
		}
		if *updatedLimits.StorageLimit > s.config.UsageLimits.Storage.Paid && *updatedLimits.StorageLimit > *project.StorageLimit {
			return ErrInvalidProjectLimit.New("Specified storage limit exceeds allowed maximum for current tier")
		}

		if !allowZero || *updatedLimits.StorageLimit != 0 {
			storageUsed, err := s.projectUsage.GetProjectStorageTotals(ctx, project.ID)
			if err != nil {
				return Error.Wrap(err)
			}
			if updatedLimits.StorageLimit.Int64() < storageUsed {
				return ErrInvalidProjectLimit.New("Cannot set storage limit below current usage")
			}
		}
	}

	if updatedLimits.BandwidthLimit != nil {
		if !allowZero && *updatedLimits.BandwidthLimit <= 0 {
			return ErrInvalidProjectLimit.New("Project limits must be greater than 0")
		}
		if project.BandwidthLimit != nil && *project.BandwidthLimit == 0 {
			return Error.New("current bandwidth limit for project is set to 0 (updating disabled)")
		}
		if *updatedLimits.BandwidthLimit > s.config.UsageLimits.Bandwidth.Paid && *updatedLimits.BandwidthLimit > *project.BandwidthLimit {
			return ErrInvalidProjectLimit.New("Specified bandwidth limit exceeds allowed maximum for current tier")
		}
		if !allowZero || *updatedLimits.BandwidthLimit != 0 {
			bandwidthUsed, err := s.projectUsage.GetProjectBandwidthTotals(ctx, project.ID)
			if err != nil {
				return Error.Wrap(err)
			}
			if updatedLimits.BandwidthLimit.Int64() < bandwidthUsed {
				return ErrInvalidProjectLimit.New("Cannot set bandwidth limit below current usage")
			}
		}
	}

	return nil
}

// MigrateProjectPricing is a method for migrating project pricing to new model.
func (s *Service) MigrateProjectPricing(ctx context.Context, publicProjectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "migrate project pricing")
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, publicProjectID)
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}
	if isMember.membership.Role != RoleAdmin {
		return ErrForbidden.New("only project owner or admin may migrate project pricing")
	}

	if !s.entitlementsConfig.Enabled || s.legacyPlacements == nil {
		return ErrForbidden.New("project pricing migration is not available")
	}

	p := isMember.project

	if ent, err := s.entitlementsService.Projects().GetByPublicID(ctx, p.PublicID); err == nil && ent.NewBucketPlacements != nil {
		if !slices.Equal(ent.NewBucketPlacements, s.legacyPlacements) {
			return ErrConflict.New("project pricing migration is only available for classic projects")
		}
	}

	partnerMap, defaultMap := s.accounts.GetPlacementProductMappings(string(p.UserAgent))

	mapping := entitlements.PlacementProductMappings{}
	for placement, productID := range partnerMap {
		mapping[storj.PlacementConstraint(placement)] = productID
	}
	for placement, productID := range defaultMap {
		if _, exists := mapping[storj.PlacementConstraint(placement)]; !exists {
			mapping[storj.PlacementConstraint(placement)] = productID
		}
	}
	for placement, productID := range s.config.LegacyPlacementProductMappingForMigration.mappings {
		mapping[placement] = productID
	}

	feats := entitlements.ProjectFeatures{
		NewBucketPlacements:      s.config.Placement.AllowedPlacementIdsForNewProjects,
		PlacementProductMappings: mapping,
	}
	featBytes, err := json.Marshal(feats)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = s.store.Entitlements().UpsertByScope(ctx, &entitlements.Entitlement{
		Scope:     entitlements.ConvertPublicIDToProjectScope(p.PublicID),
		Features:  featBytes,
		UpdatedAt: s.nowFn(),
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// RequestLimitIncrease is a method for requesting limit increase for a project.
func (s *Service) RequestLimitIncrease(ctx context.Context, projectID uuid.UUID, info LimitRequestInfo) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "request limit increase", zap.String("projectID", projectID.String()))
	if err != nil {
		return Error.Wrap(err)
	}

	_, project, err := s.isProjectOwner(ctx, user.ID, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	s.analytics.TrackRequestLimitIncrease(user.ID, user.Email, analytics.LimitRequestInfo{
		ProjectName:  project.Name,
		LimitType:    info.LimitType,
		CurrentLimit: info.CurrentLimit.String(),
		DesiredLimit: info.DesiredLimit.String(),
	}, user.HubspotObjectID, user.TenantID)

	return nil
}

// RequestProjectLimitIncrease is a method for requesting to increase max number of projects for a user.
func (s *Service) RequestProjectLimitIncrease(ctx context.Context, limit string) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "request project limit increase")
	if err != nil {
		return Error.Wrap(err)
	}

	if user.IsFreeOrMember() {
		return ErrNotPaidTier.New("Only Pro users may request project limit increases")
	}

	limitInt, err := strconv.Atoi(limit)
	if err != nil {
		return ErrInvalidProjectLimit.New("Requested project limit must be an integer")
	}

	if limitInt <= user.ProjectLimit {
		return ErrInvalidProjectLimit.New("Requested project limit (%d) must be greater than current limit (%d)", limitInt, user.ProjectLimit)
	}

	s.analytics.TrackRequestLimitIncrease(user.ID, user.Email, analytics.LimitRequestInfo{
		LimitType:    "projects",
		CurrentLimit: strconv.Itoa(user.ProjectLimit),
		DesiredLimit: limit,
	}, user.HubspotObjectID, user.TenantID)

	return nil
}

// GenUpdateProject is a method for updating project name and description by id for generated api.
func (s *Service) GenUpdateProject(ctx context.Context, projectID uuid.UUID, projectInfo UpsertProjectInfo) (p *Project, httpError api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "update project name and description", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	err = ValidateNameAndDescription(projectInfo.Name, projectInfo.Description)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.Wrap(err),
		}
	}

	project := isMember.project
	project.Name = projectInfo.Name
	project.Description = projectInfo.Description

	if user.HasPaidPrivileges() && projectInfo.StorageLimit != nil && projectInfo.BandwidthLimit != nil {
		if project.BandwidthLimit != nil && *project.BandwidthLimit == 0 {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.New("current bandwidth limit for project is set to 0 (updating disabled)"),
			}
		}
		if project.StorageLimit != nil && *project.StorageLimit == 0 {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.New("current storage limit for project is set to 0 (updating disabled)"),
			}
		}
		if *projectInfo.StorageLimit <= 0 || *projectInfo.BandwidthLimit <= 0 {
			return nil, api.HTTPError{
				Status: http.StatusBadRequest,
				Err:    Error.New("project limits must be greater than 0"),
			}
		}

		if *projectInfo.StorageLimit > s.config.UsageLimits.Storage.Paid && *projectInfo.StorageLimit > *project.StorageLimit {
			return nil, api.HTTPError{
				Status: http.StatusBadRequest,
				Err:    Error.New("specified storage limit exceeds allowed maximum for current tier"),
			}
		}

		if *projectInfo.BandwidthLimit > s.config.UsageLimits.Bandwidth.Paid && *projectInfo.BandwidthLimit > *project.BandwidthLimit {
			return nil, api.HTTPError{
				Status: http.StatusBadRequest,
				Err:    Error.New("specified bandwidth limit exceeds allowed maximum for current tier"),
			}
		}

		storageUsed, err := s.projectUsage.GetProjectStorageTotals(ctx, projectID)
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
		if projectInfo.StorageLimit.Int64() < storageUsed {
			return nil, api.HTTPError{
				Status: http.StatusBadRequest,
				Err:    Error.New("cannot set storage limit below current usage"),
			}
		}

		bandwidthUsed, err := s.projectUsage.GetProjectBandwidthTotals(ctx, projectID)
		if err != nil {
			return nil, api.HTTPError{
				Status: http.StatusInternalServerError,
				Err:    Error.Wrap(err),
			}
		}
		if projectInfo.BandwidthLimit.Int64() < bandwidthUsed {
			return nil, api.HTTPError{
				Status: http.StatusBadRequest,
				Err:    Error.New("cannot set bandwidth limit below current usage"),
			}
		}

		project.UserSpecifiedStorageLimit = projectInfo.StorageLimit
		project.UserSpecifiedBandwidthLimit = projectInfo.BandwidthLimit
	}

	err = s.store.Projects().Update(ctx, project)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	return project, httpError
}

// AddProjectMembers adds users by email to given project.
// Email addresses not belonging to a user are ignored.
// projectID here may be project.PublicID or project.ID.
func (s *Service) AddProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (users []*User, err error) {
	defer mon.Task()(&ctx)(&err)
	user, err := s.getUserAndAuditLog(ctx, "add project members", zap.String("projectID", projectID.String()), zap.Strings("emails", emails))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	// collect user querying errors
	for _, email := range emails {
		user, err := s.store.Users().GetByEmailAndTenant(ctx, email, user.TenantID)
		if err == nil {
			users = append(users, user)
		} else if !errs.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}
	}

	// add project members in transaction scope
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		for _, user := range users {
			if _, err := tx.ProjectMembers().Insert(ctx, user.ID, isMember.project.ID, RoleMember); err != nil {
				if dbx.IsConstraintError(err) {
					return errs.New("%s is already on the project", user.Email)
				}
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	s.analytics.TrackProjectMemberAddition(user.ID, user.Email, user.HubspotObjectID, user.TenantID)

	return users, nil
}

// DeleteProjectMembersAndInvitations removes users and invitations by email from given project.
// projectID here may be project.PublicID or project.ID.
func (s *Service) DeleteProjectMembersAndInvitations(ctx context.Context, projectID uuid.UUID, emails []string) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "delete project members", zap.String("projectID", projectID.String()), zap.Strings("emails", emails))
	if err != nil {
		return Error.Wrap(err)
	}

	var isMember isProjectMember
	if isMember, err = s.isProjectMember(ctx, user.ID, projectID); err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	if isMember.membership.Role != RoleAdmin {
		// We still allow user to remove themselves even with Member role.
		if len(emails) != 1 || user.Email != emails[0] {
			return ErrForbidden.New("only project Owner or Admin can remove other members")
		}
	}

	projectID = isMember.project.ID

	var userIDs []uuid.UUID
	var invitedEmails []string

	for _, email := range emails {
		invite, err := s.store.ProjectInvitations().Get(ctx, projectID, email)
		if err == nil {
			invitedEmails = append(invitedEmails, email)
			continue
		}
		if !errs.Is(err, sql.ErrNoRows) {
			return Error.Wrap(err)
		}

		user, err := s.store.Users().GetByEmailAndTenant(ctx, email, user.TenantID)
		if err != nil {
			if invite == nil {
				return ErrValidation.New(teamMemberDoesNotExistErrMsg, email)
			}
			invitedEmails = append(invitedEmails, email)
			continue
		}

		isOwner, _, err := s.isProjectOwner(ctx, user.ID, projectID)
		if isOwner {
			return ErrValidation.New(projectOwnerDeletionForbiddenErrMsg, user.Email)
		}
		if err != nil && !ErrUnauthorized.Has(err) {
			return Error.Wrap(err)
		}

		userIDs = append(userIDs, user.ID)
	}

	// delete project members in transaction scope
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) (err error) {
		for _, uID := range userIDs {
			err = tx.ProjectMembers().Delete(ctx, uID, projectID)
			if err != nil {
				return err
			}
		}
		for _, email := range invitedEmails {
			err = tx.ProjectInvitations().Delete(ctx, projectID, email)
			if err != nil {
				return err
			}
		}
		return nil
	})

	s.analytics.TrackProjectMemberDeletion(user.ID, user.Email, user.HubspotObjectID, user.TenantID)

	return Error.Wrap(err)
}

// UpdateProjectMemberRole updates project member's role and returns an updated one.
func (s *Service) UpdateProjectMemberRole(ctx context.Context, memberID, projectID uuid.UUID, newRole ProjectMemberRole) (pm *ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "update project member role", zap.String("projectID", projectID.String()), zap.String("updatedMemberID", memberID.String()), zap.String("newRole", newRole.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	_, pr, err := s.isProjectOwner(ctx, user.ID, projectID)
	if err != nil {
		if ErrUnauthorized.Has(err) {
			return nil, ErrForbidden.Wrap(errs.New("only project owners can change the role"))
		}

		return nil, Error.Wrap(err)
	}

	if pr.OwnerID == memberID {
		return nil, ErrConflict.Wrap(errs.New("project owner's status can't be changed"))
	}

	_, err = s.isProjectMember(ctx, memberID, projectID)
	if err != nil {
		return nil, ErrNoMembership.Wrap(err)
	}

	pm, err = s.store.ProjectMembers().UpdateRole(ctx, memberID, pr.ID, newRole)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return pm, err
}

// GetProjectMember queries and returns project member by given project and member IDs.
func (s *Service) GetProjectMember(ctx context.Context, memberID, projectID uuid.UUID) (pm *ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get project member", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	member, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrNoMembership.Wrap(err)
	}

	pm, err = s.store.ProjectMembers().GetByMemberIDAndProjectID(ctx, memberID, member.project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return pm, err
}

// GetProjectMembersAndInvitations returns the project members and invitations for a given project.
func (s *Service) GetProjectMembersAndInvitations(ctx context.Context, projectID uuid.UUID, cursor ProjectMembersCursor) (pmp *ProjectMembersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get project members", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}

	pmp, err = s.store.ProjectMembers().GetPagedWithInvitationsByProjectID(ctx, projectID, cursor)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// CreateDomain creates new domain.
func (s *Service) CreateDomain(ctx context.Context, domain Domain) (created *Domain, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "create domain", zap.String("projectPublicID", domain.ProjectPublicID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, domain.ProjectPublicID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	project := isMember.project

	kind := user.Kind
	if project.OwnerID != user.ID {
		kind, err = s.store.Users().GetUserKind(ctx, project.OwnerID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	if kind == FreeUser {
		return nil, ErrNotPaidTier.New("Only Pro users may create domains")
	}

	domain.ProjectID = project.ID
	domain.CreatedBy = user.ID

	created, err = s.store.Domains().Create(ctx, domain)
	return created, Error.Wrap(err)
}

// DeleteDomain deletes a domain.
func (s *Service) DeleteDomain(ctx context.Context, projectID uuid.UUID, subdomain string) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "delete domain", zap.String("projectPublicID", projectID.String()))
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	membership, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	pid := membership.project.ID

	// If not project owner or admin, make sure the user is the creator of the domain.
	if membership.project.OwnerID != user.ID && membership.membership.Role != RoleAdmin {
		domain, err := s.store.Domains().GetByProjectIDAndSubdomain(ctx, pid, subdomain)
		if err != nil {
			return err
		}
		if domain.CreatedBy != user.ID {
			return ErrForbidden.New("only project owner, admin, or the creator can delete this domain")
		}
	}

	return Error.Wrap(s.store.Domains().Delete(ctx, pid, subdomain))
}

// ListDomains returns paged domains list for a given Project.
func (s *Service) ListDomains(ctx context.Context, projectID uuid.UUID, cursor DomainCursor) (page *DomainPage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "list domains", zap.String("projectPublicID", projectID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}

	page, err = s.store.Domains().GetPagedByProjectID(ctx, isMember.project.ID, cursor)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return page, Error.Wrap(err)
}

// GetAllDomainNames returns all domain names for a given Project.
func (s *Service) GetAllDomainNames(ctx context.Context, projectID uuid.UUID) (names []string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get all domain names", zap.String("projectPublicID", projectID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	names, err = s.store.Domains().GetAllDomainNamesByProjectID(ctx, isMember.project.ID)
	return names, Error.Wrap(err)
}

// CreateAPIKey creates new api key.
// projectID here may be project.PublicID or project.ID.
func (s *Service) CreateAPIKey(ctx context.Context, projectID uuid.UUID, name string, version macaroon.APIKeyVersion) (_ *APIKeyInfo, _ *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "create api key", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, nil, ErrUnauthorized.Wrap(err)
	}

	_, err = s.store.APIKeys().GetByNameAndProjectID(ctx, name, isMember.project.ID)
	if err == nil {
		return nil, nil, ErrValidation.New(apiKeyWithNameExistsErrMsg)
	}

	secret, err := macaroon.NewSecret()
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	key, err := macaroon.NewAPIKey(secret)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	apikey := APIKeyInfo{
		Name:      name,
		ProjectID: isMember.project.ID,
		CreatedBy: user.ID,
		Secret:    secret,
		UserAgent: user.UserAgent,
		Version:   version,
	}

	info, err := s.store.APIKeys().Create(ctx, key.Head(), apikey)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return info, key, nil
}

// ProjectSupportsAuditableAPIKeys checks if the project ID is in the list of projects that support auditable API keys.
func (s *Service) ProjectSupportsAuditableAPIKeys(projectID uuid.UUID) (supports bool) {
	_, supports = s.auditableAPIKeyProjects[projectID.String()]
	return supports
}

// GenCreateAPIKey creates new api key for generated api.
func (s *Service) GenCreateAPIKey(ctx context.Context, requestInfo CreateAPIKeyRequest) (*CreateAPIKeyResponse, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "create api key", zap.String("projectID", requestInfo.ProjectID))
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	reqProjectID, err := uuid.FromString(requestInfo.ProjectID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    Error.Wrap(err),
		}
	}

	isMember, err := s.isProjectMember(ctx, user.ID, reqProjectID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	projectID := isMember.project.ID

	_, err = s.store.APIKeys().GetByNameAndProjectID(ctx, requestInfo.Name, projectID)
	if err == nil {
		return nil, api.HTTPError{
			Status: http.StatusConflict,
			Err:    ErrValidation.New(apiKeyWithNameExistsErrMsg),
		}
	}

	secret, err := macaroon.NewSecret()
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	key, err := macaroon.NewAPIKey(secret)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	apikey := APIKeyInfo{
		Name:      requestInfo.Name,
		ProjectID: projectID,
		Secret:    secret,
		UserAgent: user.UserAgent,
	}

	info, err := s.store.APIKeys().Create(ctx, key.Head(), apikey)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	// in case the project ID from the request is the public ID, replace projectID with reqProjectID
	info.ProjectID = reqProjectID

	return &CreateAPIKeyResponse{
		Key:     key.Serialize(),
		KeyInfo: info,
	}, api.HTTPError{}
}

// GenDeleteAPIKey deletes api key for generated api.
func (s *Service) GenDeleteAPIKey(ctx context.Context, keyID uuid.UUID) (httpError api.HTTPError) {
	err := s.DeleteAPIKeys(ctx, []uuid.UUID{keyID})
	if err != nil {
		if errs.Is(err, sql.ErrNoRows) {
			return httpError
		}

		status := http.StatusInternalServerError
		if ErrUnauthorized.Has(err) {
			status = http.StatusUnauthorized
		} else if ErrAPIKeyRequest.Has(err) {
			status = http.StatusBadRequest
		}

		return api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	return httpError
}

// GenGetAPIKeys returns api keys belonging to a project for generated api.
func (s *Service) GenGetAPIKeys(ctx context.Context, projectID uuid.UUID, search string, limit, page uint, order APIKeyOrder, orderDirection OrderDirection) (*APIKeyPage, api.HTTPError) {
	akp, err := s.GetAPIKeys(ctx, projectID, APIKeyCursor{
		Search:         search,
		Limit:          limit,
		Page:           page,
		Order:          order,
		OrderDirection: orderDirection,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if ErrUnauthorized.Has(err) {
			status = http.StatusUnauthorized
		} else if ErrAPIKeyRequest.Has(err) {
			status = http.StatusBadRequest
		}

		return nil, api.HTTPError{
			Status: status,
			Err:    Error.Wrap(err),
		}
	}

	return akp, api.HTTPError{}
}

// GetAPIKeyInfoByName retrieves an api key by its name and project id.
func (s *Service) GetAPIKeyInfoByName(ctx context.Context, projectID uuid.UUID, name string) (_ *APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get api key info",
		zap.String("projectID", projectID.String()),
		zap.String("name", name))
	if err != nil {
		return nil, err
	}

	key, err := s.store.APIKeys().GetByNameAndProjectID(ctx, name, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, user.ID, key.ProjectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	return key, nil
}

// GetAPIKeyInfo retrieves api key by id.
func (s *Service) GetAPIKeyInfo(ctx context.Context, id uuid.UUID) (_ *APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get api key info", zap.String("apiKeyID", id.String()))
	if err != nil {
		return nil, err
	}

	key, err := s.store.APIKeys().Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, user.ID, key.ProjectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	return key, nil
}

// DeleteAPIKeys deletes api key by id.
func (s *Service) DeleteAPIKeys(ctx context.Context, ids []uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	idStrings := make([]string, 0, len(ids))
	for _, id := range ids {
		idStrings = append(idStrings, id.String())
	}

	user, err := s.getUserAndAuditLog(ctx, "delete api keys", zap.Strings("apiKeyIDs", idStrings))
	if err != nil {
		return Error.Wrap(err)
	}

	var keysErr errs.Group

	for _, keyID := range ids {
		key, err := s.store.APIKeys().Get(ctx, keyID)
		if err != nil {
			keysErr.Add(err)
			continue
		}

		pm, err := s.isProjectMember(ctx, user.ID, key.ProjectID)
		if err != nil {
			keysErr.Add(ErrUnauthorized.Wrap(err))
			continue
		}

		if pm.membership.Role != RoleAdmin && key.CreatedBy != pm.membership.MemberID {
			keysErr.Add(ErrForbidden.Wrap(errs.New("you do not have permission to delete this API key: %s", key.Name)))
			continue
		}
	}

	if err = keysErr.Err(); err != nil {
		return Error.Wrap(err)
	}

	err = s.store.APIKeys().DeleteMultiple(ctx, ids)
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// GetAllAPIKeyNamesByProjectID returns all api key names by project ID.
func (s *Service) GetAllAPIKeyNamesByProjectID(ctx context.Context, projectID uuid.UUID) (names []string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get all api key names by project ID", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	names, err = s.store.APIKeys().GetAllNamesByProjectID(ctx, isMember.project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return names, nil
}

// DeleteAPIKeyByNameAndProjectID deletes api key by name and project ID.
// ID here may be project.publicID or project.ID.
func (s *Service) DeleteAPIKeyByNameAndProjectID(ctx context.Context, name string, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "delete api key by name and project ID", zap.String("apiKeyName", name), zap.String("projectID", projectID.String()))
	if err != nil {
		return Error.Wrap(err)
	}

	pm, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	key, err := s.store.APIKeys().GetByNameAndProjectID(ctx, name, pm.project.ID)
	if err != nil {
		return ErrNoAPIKey.New(apiKeyWithNameDoesntExistErrMsg)
	}

	if pm.membership.Role != RoleAdmin && key.CreatedBy != pm.membership.MemberID {
		return ErrForbidden.Wrap(errs.New("you do not have permission to delete this API key"))
	}

	err = s.store.APIKeys().Delete(ctx, key.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetAPIKeys returns paged api key list for given Project.
func (s *Service) GetAPIKeys(ctx context.Context, reqProjectID uuid.UUID, cursor APIKeyCursor) (page *APIKeyPage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get api keys", zap.String("projectID", reqProjectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, reqProjectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	projectID := isMember.project.ID

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}

	page, err = s.store.APIKeys().GetPagedByProjectID(ctx, projectID, cursor, s.config.ObjectBrowserKeyNamePrefix)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// if project ID from request is public ID, replace api key's project IDs with public ID
	if projectID != reqProjectID {
		for i := range page.APIKeys {
			page.APIKeys[i].ProjectID = reqProjectID
		}
	}

	return page, err
}

// GetProjectUsage retrieves project usage for a given period.
func (s *Service) GetProjectUsage(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ *accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get project usage", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	projectUsage, err := s.projectAccounting.GetProjectTotal(ctx, projectID, since, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return projectUsage, nil
}

// getLocationName returns the product short name if available, otherwise the placement name.
func (s *Service) getLocationName(ctx context.Context, projectPublicID uuid.UUID, placementID storj.PlacementConstraint) string {
	// Check if showNewPricingTiers is enabled and we have product configs
	if s.config.ShowNewPricingTiers && s.productConfigs != nil {
		var productID int32
		var found bool

		// First, check per-project entitlements if enabled
		if s.entitlementsConfig.Enabled && s.entitlementsService != nil {
			features, err := s.entitlementsService.Projects().GetByPublicID(ctx, projectPublicID)
			if err == nil && features.PlacementProductMappings != nil {
				if pid, ok := features.PlacementProductMappings[placementID]; ok {
					productID = pid
					found = true
				}
			}
		}

		// Fall back to global placement product map if no entitlement mapping found
		if !found && s.placementProductMap != nil {
			if pid, ok := s.placementProductMap[int(placementID)]; ok {
				productID = pid
				found = true
			}
		}

		// If we found a product mapping, look up the product configuration
		if found {
			if product, ok := s.productConfigs[productID]; ok && product.ProductShortName != "" {
				return product.ProductShortName
			}
		}
	}

	// Fall back to placement name
	placement, ok := s.placements[placementID]
	if !ok {
		return fmt.Sprintf("unknown(%d)", placementID)
	}
	return placement.Name
}

// GetBucketTotals retrieves paged bucket total usages since project creation.
func (s *Service) GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor accounting.BucketUsageCursor, since, before time.Time) (_ *accounting.BucketUsagePage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get bucket totals", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	usage, err := s.projectAccounting.GetBucketTotals(ctx, isMember.project.ID, cursor, since, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if usage == nil {
		return usage, nil
	}

	for i := range usage.BucketUsages {
		placementID := usage.BucketUsages[i].DefaultPlacement
		usage.BucketUsages[i].Location = s.getLocationName(ctx, isMember.project.PublicID, placementID)
	}

	return usage, nil
}

// GetSingleBucketTotals retrieves a single bucket total usages since project creation.
func (s *Service) GetSingleBucketTotals(ctx context.Context, projectID uuid.UUID, bucketName string, before time.Time) (_ *accounting.BucketUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get single bucket totals", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	usage, err := s.projectAccounting.GetSingleBucketTotals(ctx, isMember.project.ID, bucketName, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	usage.Location = s.getLocationName(ctx, isMember.project.PublicID, usage.DefaultPlacement)

	return usage, nil
}

// GetAllBucketNames retrieves all bucket names of a specific project.
// projectID here may be Project.ID or Project.PublicID.
func (s *Service) GetAllBucketNames(ctx context.Context, projectID uuid.UUID) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get all bucket names", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	listOptions := buckets.ListOptions{
		Direction: buckets.DirectionForward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	bucketsList, err := s.buckets.ListBuckets(ctx, isMember.project.ID, listOptions, allowedBuckets)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var list []string
	for _, bucket := range bucketsList.Items {
		list = append(list, bucket.Name)
	}

	return list, nil
}

// GetBucketMetadata retrieves all bucket names of a specific project and related metadata, e.g. placement and versioning.
// projectID here may be Project.ID or Project.PublicID.
func (s *Service) GetBucketMetadata(ctx context.Context, projectID uuid.UUID) (list []BucketMetadata, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get all bucket names and metadata", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	listOptions := buckets.ListOptions{
		Direction: buckets.DirectionForward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	bucketsList, err := s.buckets.ListBuckets(ctx, isMember.project.ID, listOptions, allowedBuckets)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, bucket := range bucketsList.Items {
		list = append(list, BucketMetadata{
			Name:       bucket.Name,
			Versioning: bucket.Versioning,
			Placement: Placement{
				DefaultPlacement: bucket.Placement,
				Location:         s.getLocationName(ctx, isMember.project.PublicID, bucket.Placement),
			},
			ObjectLockEnabled: bucket.ObjectLock.Enabled,
		})
	}

	return list, nil
}

// GetPlacementDetails retrieves all placement with human-readable details available to a project's user agent.
func (s *Service) GetPlacementDetails(ctx context.Context, projectID uuid.UUID) (_ []PlacementDetail, err error) {
	user, err := GetUser(ctx)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}
	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	project := isMember.project

	return s.getPlacementDetails(ctx, project)
}

func (s *Service) getPlacementDetails(ctx context.Context, project *Project) ([]PlacementDetail, error) {
	placements, entitlementsHasPlacements, err := s.accounts.GetPartnerPlacements(ctx, project.PublicID, string(project.UserAgent))
	if err != nil {
		return nil, err
	}

	if project.DefaultPlacement != storj.DefaultPlacement {
		if !s.entitlementsConfig.Enabled {
			// if entitlements are disabled, projects can only use self serve placements
			// if they have a zero default placement.
			return []PlacementDetail{}, nil
		}

		if s.entitlementsConfig.Enabled && !entitlementsHasPlacements {
			// in this case, the project has no placements available via entitlements, so placements
			// is now the global defaults. But a non-default default placement means the project
			// has no access to the global self-serve placements.
			return []PlacementDetail{}, nil
		}
	}

	details := make([]PlacementDetail, 0)
	for _, placement := range placements {
		if detail, ok := s.config.Placement.SelfServeDetails.Get(placement); ok {
			details = append(details, detail)
		}
	}
	if len(details) == 1 && details[0].ID == int(project.DefaultPlacement) {
		// if the only placement available is the default placement,
		// don't return any placement details.
		return []PlacementDetail{}, nil
	}
	return details, nil
}

// GetUsageReportParam contains parameters for GetUsageReport method.
type GetUsageReportParam struct {
	Since, Before  time.Time
	ProjectID      uuid.UUID
	GroupByProject bool
	IncludeCost    bool
}

// GetUsageReport retrieves usage rollups for every bucket of a single or all the user owned projects for a given period.
func (s *Service) GetUsageReport(ctx context.Context, param GetUsageReportParam) ([]accounting.ProjectReportItem, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get usage report")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var projects []Project

	if param.ProjectID.IsZero() {
		pr, err := s.store.Projects().GetOwnActive(ctx, user.ID)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		projects = append(projects, pr...)
	} else {
		_, pr, err := s.isProjectOwner(ctx, user.ID, param.ProjectID)
		if err != nil {
			return nil, ErrUnauthorized.Wrap(err)
		}

		projects = append(projects, *pr)
	}

	reportUsages := make([]accounting.ProjectReportItem, 0)

	for _, p := range projects {
		if !param.GroupByProject || !s.config.NewDetailedUsageReportEnabled {
			rollups, err := s.projectAccounting.GetBucketUsageRollups(ctx, p.ID, param.Since, param.Before, true)
			if err != nil {
				return nil, Error.Wrap(err)
			}

			for _, r := range rollups {
				item := accounting.ProjectReportItem{
					ProjectName:     p.Name,
					ProjectPublicID: p.PublicID,
					ProjectID:       p.ID,
					BucketName:      r.BucketName,
					Storage:         r.TotalStoredData,
					Egress:          r.GetEgress,
					ObjectCount:     r.ObjectCount,
					SegmentCount:    r.TotalSegments,
					Since:           r.Since,
					Before:          r.Before,
					Placement:       r.Placement,
					UserAgent:       r.UserAgent,
				}

				if s.config.NewDetailedUsageReportEnabled {
					item, err = s.transformProjectReportItem(ctx, item, param.IncludeCost, payments.ProductUsagePriceModel{})
					if err != nil {
						return nil, Error.Wrap(err)
					}
				}

				reportUsages = append(reportUsages, item)
			}
		} else {
			usages, err := s.projectAccounting.GetProjectTotalByPartnerAndPlacement(ctx, p.ID, s.accounts.GetPartnerNames(), param.Since, param.Before, false)
			if err != nil {
				return nil, err
			}

			for key, usage := range usages {
				usage.Storage = memory.Size(usage.Storage).GB()
				usage.Egress = int64(memory.Size(usage.Egress).GB())

				item := accounting.ProjectReportItem{
					ProjectName:     p.Name,
					ProjectPublicID: p.PublicID,
					ProjectID:       p.ID,
					Egress:          float64(usage.Egress),
					Storage:         usage.Storage,
					SegmentCount:    usage.SegmentCount,
					ObjectCount:     usage.ObjectCount,
					UserAgent:       p.UserAgent,
					Since:           param.Since,
					Before:          param.Before,
				}

				_, priceModel := s.accounts.ProductIdAndPriceForUsageKey(ctx, p.PublicID, key)

				partner := ""
				placement := int(storj.DefaultPlacement)

				// Split the key to extract partner and placement.
				parts := strings.Split(key, "|")
				if len(parts) >= 1 {
					partner = parts[0]
				}
				if len(parts) >= 2 {
					placement64, err := strconv.ParseInt(parts[1], 10, 32)
					if err == nil {
						placement = int(placement64)
					}
				}
				item.Placement = storj.PlacementConstraint(placement)
				item.UserAgent = []byte(partner)

				item, err = s.transformProjectReportItem(ctx, item, param.IncludeCost, priceModel)
				if err != nil {
					return nil, Error.Wrap(err)
				}
				reportUsages = append(reportUsages, item)
			}
		}
	}

	return reportUsages, nil
}

// GetReportRow converts the report item into a row for the usage report.
func (s *Service) GetReportRow(param GetUsageReportParam, reportItem accounting.ProjectReportItem) []string {
	if !s.config.NewDetailedUsageReportEnabled {
		return []string{
			reportItem.ProjectName,
			reportItem.ProjectPublicID.String(),
			reportItem.BucketName,
			fmt.Sprintf("%f", reportItem.Storage),
			fmt.Sprintf("%f", reportItem.Egress),
			fmt.Sprintf("%f", reportItem.ObjectCount),
			fmt.Sprintf("%f", reportItem.SegmentCount),
			reportItem.Since.String(),
			reportItem.Before.String(),
		}
	}
	row := []string{
		reportItem.ProjectName,
		reportItem.ProjectPublicID.String(),
	}
	if !param.GroupByProject {
		row = append(row, reportItem.BucketName)
	}
	if s.config.SkuEnabled {
		row = append(row, reportItem.StorageSKU)
	}
	row = append(row, fmt.Sprintf("%f", reportItem.Storage))
	row = append(row, fmt.Sprintf("%f", reportItem.StorageTbMonth))
	if param.IncludeCost {
		row = append(row, fmt.Sprintf("%.2f", reportItem.StorageCost/100))
	}
	if s.config.SkuEnabled {
		row = append(row, reportItem.EgressSKU)
	}
	row = append(row, fmt.Sprintf("%f", reportItem.Egress))
	row = append(row, fmt.Sprintf("%f", reportItem.EgressTb))
	if param.IncludeCost {
		row = append(row, fmt.Sprintf("%.2f", reportItem.EgressCost/100))
	}
	row = append(row, fmt.Sprintf("%f", reportItem.ObjectCount))
	if s.config.SkuEnabled {
		row = append(row, reportItem.SegmentSKU)
	}
	row = append(row, fmt.Sprintf("%f", reportItem.SegmentCount))
	row = append(row, fmt.Sprintf("%f", reportItem.SegmentCountMonth))
	if param.IncludeCost {
		row = append(row, fmt.Sprintf("%.2f", reportItem.SegmentCost/100))
		row = append(row, fmt.Sprintf("%.2f", reportItem.TotalCost/100))
	}
	row = append(row, reportItem.Since.String())
	row = append(row, reportItem.Before.String())

	return row
}

// GetUsageReportHeaders returns headers for the usage report. It includes a disclaimer for pricing if
// the new detailed usage report is enabled and cost is requested.
func (s *Service) GetUsageReportHeaders(param GetUsageReportParam) (disclaimer []string, headers []string) {
	if !s.config.NewDetailedUsageReportEnabled {
		return nil, []string{
			"ProjectName", "ProjectID", "BucketName", "Storage GB-hour", "Egress GB",
			"ObjectCount objects-hour", "SegmentCount segments-hour", "Since", "Before",
		}
	}
	headers = []string{
		"ProjectName", "ProjectID", "BucketName", "Storage SKU", "Storage GB-hour", "Storage TB-months",
		"Estimated Storage Price ($)", "Egress SKU", "Egress GB", "Egress TB", "Estimated Egress Price ($)",
		"ObjectCount objects-hour", "Segment SKU", "SegmentCount segments-hour", "Segment Months",
		"Estimated Segment Price ($)", "Estimated Total Amount ($)", "Since", "Before",
	}

	if !s.config.SkuEnabled {
		updateHeaders := make([]string, 0, len(headers)-4)
		for _, header := range headers {
			if strings.Contains(header, "SKU") {
				continue
			}
			updateHeaders = append(updateHeaders, header)
		}
		headers = updateHeaders
	}
	if param.GroupByProject {
		headerSlice := headers[:2]
		headers = append(headerSlice, headers[3:]...)
	}
	if !param.IncludeCost {
		updateHeaders := make([]string, 0, len(headers)-4)
		for _, header := range headers {
			if strings.Contains(header, "Estimated") {
				continue
			}
			updateHeaders = append(updateHeaders, header)
		}
		headers = updateHeaders
	}

	if param.IncludeCost {
		disclaimer = []string{"Disclaimer: The actual billed amount may differ due to custom billing, discounts, or coupons applied at the time of invoicing."}
		// append empty columns so that disclaimerRow is the same length as csvHeaders
		disclaimer = append(disclaimer, make([]string, len(headers)-1)...)
	}

	return disclaimer, headers
}

// transformProjectReportItem modifies the project report item, converting GB values to TB and
// hour values to month values. It includes cost if addCost is true.
func (s *Service) transformProjectReportItem(ctx context.Context, item accounting.ProjectReportItem, addCost bool, priceModel payments.ProductUsagePriceModel) (_ accounting.ProjectReportItem, err error) {
	hoursPerMonthDecimal := decimal.NewFromInt(hoursPerMonth)
	if priceModel == (payments.ProductUsagePriceModel{}) {
		_, priceModel = s.accounts.GetPartnerPlacementPriceModel(ctx, item.ProjectPublicID, string(item.UserAgent), item.Placement)
	}
	item.ProductName = priceModel.ProductName
	if s.config.SkuEnabled {
		item.StorageSKU = priceModel.StorageSKU
		item.SegmentSKU = priceModel.SegmentSKU
		item.EgressSKU = priceModel.EgressSKU
	}

	if addCost {
		// storage and egress are in GB, convert to bytes
		storageBytes, _ := decimal.NewFromFloat(item.Storage).Shift(9).Float64()
		egressBytes, _ := decimal.NewFromFloat(item.Egress).Shift(9).Float64()
		usage := accounting.ProjectUsage{
			Storage:      storageBytes,
			Egress:       int64(egressBytes),
			ObjectCount:  item.ObjectCount,
			SegmentCount: item.SegmentCount,
		}

		usageCost := s.accounts.CalculateProjectUsagePrice(usage, priceModel.ProjectUsagePriceModel)
		item.EgressCost, _ = usageCost.Egress.Float64()
		item.StorageCost, _ = usageCost.Storage.Float64()
		item.SegmentCost, _ = usageCost.Segment.Float64()
		item.TotalCost = item.EgressCost + item.StorageCost + item.SegmentCost
	}
	item.EgressTb, _ = decimal.NewFromFloat(item.Egress).Shift(-3).Float64()
	item.StorageTbMonth, _ = decimal.NewFromFloat(item.Storage).Shift(-3).Div(hoursPerMonthDecimal).Float64()
	item.SegmentCountMonth, _ = decimal.NewFromFloat(item.SegmentCount).Div(hoursPerMonthDecimal).Float64()

	return item, nil
}

// GenGetBucketUsageRollups retrieves summed usage rollups for every bucket of particular project for a given period for generated api.
func (s *Service) GenGetBucketUsageRollups(ctx context.Context, reqProjectID uuid.UUID, since, before time.Time) (rollups []accounting.BucketUsageRollup, httpError api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get bucket usage rollups", zap.String("projectID", reqProjectID.String()))
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	isMember, err := s.isProjectMember(ctx, user.ID, reqProjectID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	projectID := isMember.project.ID

	rollups, err = s.projectAccounting.GetBucketUsageRollups(ctx, projectID, since, before, false)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	// if project ID from request is public ID, replace rollup's project ID with public ID
	if reqProjectID != projectID {
		for i := range rollups {
			rollups[i].ProjectID = reqProjectID
		}
	}

	return rollups, httpError
}

// GenGetSingleBucketUsageRollup retrieves usage rollup for single bucket of particular project for a given period for generated api.
func (s *Service) GenGetSingleBucketUsageRollup(ctx context.Context, reqProjectID uuid.UUID, bucket string, since, before time.Time) (rollup *accounting.BucketUsageRollup, httpError api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get single bucket usage rollup", zap.String("projectID", reqProjectID.String()))
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	isMember, err := s.isProjectMember(ctx, user.ID, reqProjectID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusUnauthorized,
			Err:    Error.Wrap(err),
		}
	}

	projectID := isMember.project.ID

	rollup, err = s.projectAccounting.GetSingleBucketUsageRollup(ctx, projectID, bucket, since, before)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	// make sure to replace rollup project ID with reqProjectID in case it is the public ID
	rollup.ProjectID = reqProjectID

	return rollup, httpError
}

// GetDailyProjectUsage returns daily usage by project ID.
// ID here may be project.ID or project.PublicID.
func (s *Service) GetDailyProjectUsage(ctx context.Context, projectID uuid.UUID, from, to time.Time) (_ *accounting.ProjectDailyUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get daily usage by project ID")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	usage, err := s.projectAccounting.GetProjectDailyUsageByDateRange(ctx, isMember.project.ID, from, to, s.config.AsOfSystemTimeDuration)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return usage, nil
}

// GetProjectUsageLimits returns project limits and current usage.
//
// Among others,it can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Service, wrapped Error.
func (s *Service) GetProjectUsageLimits(ctx context.Context, projectID uuid.UUID) (_ *ProjectUsageLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get project usage limits", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	prUsageLimits, err := s.getProjectUsageLimits(ctx, isMember.project.ID, false)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	prObjectsSegments, err := s.projectAccounting.GetProjectObjectsSegments(ctx, isMember.project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	prUsageLimits.ObjectCount = prObjectsSegments.ObjectCount
	prUsageLimits.SegmentCount = prObjectsSegments.SegmentCount

	return prUsageLimits, nil
}

// GetTotalUsageLimits returns total limits and current usage for all the projects.
func (s *Service) GetTotalUsageLimits(ctx context.Context) (_ *ProjectUsageLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get total usage and limits for all the projects")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projects, err := s.store.Projects().GetOwnActive(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var totalStorageLimit int64
	var totalBandwidthLimit int64
	var totalStorageUsed int64
	var totalBandwidthUsed int64

	for _, pr := range projects {
		prUsageLimits, err := s.getProjectUsageLimits(ctx, pr.ID, true)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		totalStorageLimit += prUsageLimits.StorageLimit
		totalBandwidthLimit += prUsageLimits.BandwidthLimit
		totalStorageUsed += prUsageLimits.StorageUsed
		totalBandwidthUsed += prUsageLimits.BandwidthUsed
	}

	return &ProjectUsageLimits{
		StorageLimit:   totalStorageLimit,
		BandwidthLimit: totalBandwidthLimit,
		StorageUsed:    totalStorageUsed,
		BandwidthUsed:  totalBandwidthUsed,
	}, nil
}

func (s *Service) getStorageAndBandwidthUse(ctx context.Context, projectID uuid.UUID) (storage, bandwidth int64, err error) {
	defer mon.Task()(&ctx)(&err)

	storage, err = s.projectUsage.GetProjectStorageTotals(ctx, projectID)
	if err != nil {
		return 0, 0, err
	}

	now := s.nowFn()
	bandwidth, err = s.projectUsage.GetProjectBandwidth(ctx, projectID, now.Year(), now.Month(), now.Day())
	if err != nil {
		return 0, 0, err
	}

	return storage, bandwidth, nil
}

func (s *Service) getProjectUsageLimits(ctx context.Context, projectID uuid.UUID, getBandwidthTotals bool) (_ *ProjectUsageLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	limits, err := s.projectUsage.GetProjectLimits(ctx, projectID)
	if err != nil {
		return nil, err
	}

	storageUsed, segmentUsed, err := s.projectUsage.GetProjectStorageAndSegmentUsage(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var bandwidthUsed int64
	if getBandwidthTotals {
		bandwidthUsed, err = s.projectUsage.GetProjectBandwidthTotals(ctx, projectID)
	} else {
		now := s.nowFn()
		bandwidthUsed, err = s.projectUsage.GetProjectBandwidth(ctx, projectID, now.Year(), now.Month(), now.Day())
	}
	if err != nil {
		return nil, err
	}

	bucketsUsed, err := s.buckets.CountBuckets(ctx, projectID)
	if err != nil {
		return nil, err
	}

	bucketsLimit, err := s.store.Projects().GetMaxBuckets(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if bucketsLimit == nil {
		bucketsLimit = &s.maxProjectBuckets
	}

	return &ProjectUsageLimits{
		StorageLimit:          *limits.Usage,
		UserSetStorageLimit:   limits.UserSetUsage,
		BandwidthLimit:        *limits.Bandwidth,
		UserSetBandwidthLimit: limits.UserSetBandwidth,
		StorageUsed:           storageUsed,
		BandwidthUsed:         bandwidthUsed,
		SegmentLimit:          *limits.Segments,
		SegmentUsed:           segmentUsed,
		BucketsUsed:           int64(bucketsUsed),
		BucketsLimit:          int64(*bucketsLimit),
	}, nil
}

// TokenAuth returns an authenticated context by session token.
func (s *Service) TokenAuth(ctx context.Context, token consoleauth.Token, authTime time.Time) (_ context.Context, _ *consoleauth.WebappSession, err error) {
	defer mon.Task()(&ctx)(&err)

	valid, err := s.tokens.ValidateToken(token)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	if !valid {
		return nil, nil, Error.New("incorrect signature")
	}

	sessionID, err := uuid.FromBytes(token.Payload)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	session, err := s.store.WebappSessions().GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	ctx, err = s.authorize(ctx, session.UserID, session.ExpiresAt, authTime)
	if err != nil {
		err := errs.Combine(err, s.store.WebappSessions().DeleteBySessionID(ctx, sessionID))
		if err != nil {
			return nil, nil, Error.Wrap(err)
		}
		return nil, nil, err
	}

	return ctx, &session, nil
}

// KeyAuth returns an authenticated context by api key.
func (s *Service) KeyAuth(ctx context.Context, apikey string, authTime time.Time) (_ context.Context, err error) {
	defer mon.Task()(&ctx)(&err)

	ctx = consoleauth.WithAPIKey(ctx, []byte(apikey))

	userID, exp, err := s.GetUserAndExpirationFromKey(ctx, apikey)
	if err != nil {
		return nil, err
	}

	ctx, err = s.authorize(ctx, userID, exp, authTime)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

// checkProjectCanBeDeleted ensures that all data, api-keys and buckets are deleted and usage has been accounted.
// no error means the project status is clean.
func (s *Service) checkProjectCanBeDeleted(ctx context.Context, user *User, project *Project, step AccountActionStep, data string) (resp *DeleteProjectInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	if s.config.AbbreviatedDeleteProjectEnabled {
		// check for buckets with Object Lock enabled
		count, err := s.buckets.CountObjectLockBuckets(ctx, project.ID)
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return &DeleteProjectInfo{LockEnabledBuckets: count}, ErrUsage.New("some buckets with Object Lock enabled exist")
		}
	}

	if !s.config.AbbreviatedDeleteProjectEnabled {
		buckets, err := s.buckets.CountBuckets(ctx, project.ID)
		if err != nil {
			return nil, err
		}
		if buckets > 0 {
			return &DeleteProjectInfo{Buckets: buckets}, ErrUsage.New("some buckets still exist")
		}

		// ignore object browser api key because we hide it from the user, so they can't delete it.
		// project row deletion cascades to api keys, so it's okay.
		keys, err := s.store.APIKeys().GetAllNamesByProjectID(ctx, project.ID)
		if err != nil {
			return nil, err
		}

		var keyCount int
		for _, k := range keys {
			if !strings.HasPrefix(k, s.config.ObjectBrowserKeyNamePrefix) {
				keyCount++
			}
		}
		if keyCount > 0 {
			return &DeleteProjectInfo{APIKeys: keyCount}, ErrUsage.New("some api keys still exist")
		}
	}

	currentPrice := decimal.Zero

	if user.IsPaid() {
		currentUsage, invoicingIncomplete, currentMonthPrice, err := s.Payments().checkProjectUsageStatus(ctx, *project)
		if err != nil && !payments.ErrUnbilledUsage.Has(err) {
			return nil, ErrUsage.Wrap(err)
		}

		currentPrice = currentMonthPrice

		if currentUsage || invoicingIncomplete {
			return &DeleteProjectInfo{
				CurrentUsage:        currentUsage,
				InvoicingIncomplete: invoicingIncomplete,
				CurrentMonthPrice:   currentMonthPrice,
			}, ErrUsage.Wrap(err)
		}
	}

	return &DeleteProjectInfo{CurrentMonthPrice: currentPrice}, nil
}

// checkProjectLimit is used to check if user is able to create a new project.
func (s *Service) checkProjectLimit(ctx context.Context, userID uuid.UUID) (currentProjects int, err error) {
	defer mon.Task()(&ctx)(&err)

	limit, err := s.store.Users().GetProjectLimit(ctx, userID)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	projects, err := s.store.Projects().GetOwnActive(ctx, userID)
	if err != nil {
		return 0, Error.Wrap(err)
	}

	if len(projects) >= limit {
		return 0, ErrProjLimit.New(projLimitErrMsg)
	}

	return len(projects), nil
}

// checkProjectName is used to check if user has used project name before.
func (s *Service) checkProjectName(ctx context.Context, projectInfo UpsertProjectInfo, userID uuid.UUID) (passesNameCheck bool, err error) {
	defer mon.Task()(&ctx)(&err)
	passesCheck := true

	projects, err := s.store.Projects().GetOwnActive(ctx, userID)
	if err != nil {
		return false, Error.Wrap(err)
	}

	for _, project := range projects {
		if project.Name == projectInfo.Name {
			return false, ErrProjName.New(projNameErrMsg)
		}
	}

	return passesCheck, nil
}

// getUserProjectLimits is a method to get the users storage and bandwidth limits for new projects.
func (s *Service) getUserProjectLimits(ctx context.Context, userID uuid.UUID) (_ *UsageLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := s.store.Users().GetUserProjectLimits(ctx, userID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &UsageLimits{
		Storage:   result.ProjectStorageLimit.Int64(),
		Bandwidth: result.ProjectBandwidthLimit.Int64(),
		Segment:   result.ProjectSegmentLimit,
	}, nil
}

// CreateRegToken creates new registration token. Needed for testing.
func (s *Service) CreateRegToken(ctx context.Context, projLimit int) (_ *RegistrationToken, err error) {
	defer mon.Task()(&ctx)(&err)
	result, err := s.store.RegistrationTokens().Create(ctx, projLimit)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// authorize returns an authorized context by user ID.
func (s *Service) authorize(ctx context.Context, userID uuid.UUID, expiration time.Time, authTime time.Time) (_ context.Context, err error) {
	defer mon.Task()(&ctx)(&err)
	if !expiration.IsZero() && expiration.Before(authTime) {
		return nil, ErrTokenExpiration.New("authorization failed. expiration reached.")
	}

	user, err := s.store.Users().Get(ctx, userID)
	if err != nil {
		return nil, Error.New("authorization failed. no user with id: %s", userID.String())
	}

	if user.Status != Active && user.Status != PendingBotVerification {
		return nil, Error.New("authorization failed. no active user with id: %s", userID.String())
	}
	return WithUser(ctx, user), nil
}

// isProjectMember is return type of isProjectMember service method.
type isProjectMember struct {
	project    *Project
	membership *ProjectMember
}

// isProjectOwner checks if the user is an owner of a project.
func (s *Service) isProjectOwner(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (isOwner bool, project *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err = s.GetProjectNoAuth(ctx, projectID)
	if err != nil {
		return false, nil, err
	}

	if project.Status != nil && *project.Status == ProjectDisabled {
		return false, nil, errs.New(unauthorizedErrMsg)
	}

	if project.OwnerID != userID {
		return false, nil, ErrUnauthorized.New(unauthorizedErrMsg)
	}

	return true, project, nil
}

// isProjectMember checks if the user is a member of given project.
// projectID can be either private ID or public ID (project.ID/project.PublicID).
func (s *Service) isProjectMember(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (_ isProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)

	project, err := s.GetProjectNoAuth(ctx, projectID)
	if err != nil {
		return isProjectMember{}, err
	}

	if project.Status != nil && *project.Status == ProjectDisabled {
		return isProjectMember{}, errs.New(unauthorizedErrMsg)
	}

	memberships, err := s.store.ProjectMembers().GetByMemberID(ctx, userID)
	if err != nil {
		return isProjectMember{}, err
	}

	membership, ok := findMembershipByProjectID(memberships, project.ID)
	if ok {
		return isProjectMember{
			project:    project,
			membership: &membership,
		}, nil
	}

	return isProjectMember{}, ErrNoMembership.New(unauthorizedErrMsg)
}

// GetPlacementByName returns the placement constraint by name.
func (s *Service) GetPlacementByName(name string) (storj.PlacementConstraint, error) {
	if placement, ok := s.placementNameLookup[name]; ok {
		return placement, nil
	}
	return storj.DefaultPlacement, ErrPlacementNotFound.New("")
}

// WalletInfo contains all the information about a destination wallet assigned to a user.
type WalletInfo struct {
	Address blockchain.Address `json:"address"`
	Balance currency.Amount    `json:"balance"`
}

// PaymentInfo includes token payment information required by GUI.
type PaymentInfo struct {
	ID        string
	Type      string
	Wallet    string
	Amount    currency.Amount
	Received  currency.Amount
	Status    string
	Link      string
	Timestamp time.Time
}

// WalletPayments represents the list of ERC-20 token payments.
type WalletPayments struct {
	Payments []PaymentInfo `json:"payments"`
}

// BlockExplorerURL creates zkSync/etherscan transaction URI based on source.
func (payment Payments) BlockExplorerURL(tx string, source string) string {
	url := payment.service.config.BlockExplorerURL
	if source == billing.StorjScanZkSyncSource {
		url = payment.service.config.ZkSyncBlockExplorerURL
	}
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}

	return url + "tx/" + tx
}

// ErrWalletNotClaimed shows that no address is claimed by the user.
var ErrWalletNotClaimed = errs.Class("wallet is not claimed")

// TestSwapDepositWallets replaces the existing handler for deposit wallets with
// the one specified for use in testing.
func (payment Payments) TestSwapDepositWallets(dw payments.DepositWallets) {
	payment.service.depositWallets = dw
}

// ClaimWallet requests a new wallet for the users to be used for payments. If wallet is already claimed,
// it will return with the info without error.
func (payment Payments) ClaimWallet(ctx context.Context) (_ WalletInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := payment.service.getUserAndAuditLog(ctx, "claim wallet")
	if err != nil {
		return WalletInfo{}, Error.Wrap(err)
	}
	address, err := payment.service.depositWallets.Claim(ctx, user.ID)
	if err != nil {
		return WalletInfo{}, Error.Wrap(err)
	}
	balance, err := payment.service.billing.GetBalance(ctx, user.ID)
	if err != nil {
		return WalletInfo{}, Error.Wrap(err)
	}
	return WalletInfo{
		Address: address,
		Balance: balance,
	}, nil
}

// GetWallet returns with the assigned wallet, or with ErrWalletNotClaimed if not yet claimed.
func (payment Payments) GetWallet(ctx context.Context) (_ WalletInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := GetUser(ctx)
	if err != nil {
		return WalletInfo{}, Error.Wrap(err)
	}
	address, err := payment.service.depositWallets.Get(ctx, user.ID)
	if err != nil {
		return WalletInfo{}, Error.Wrap(err)
	}
	balance, err := payment.service.billing.GetBalance(ctx, user.ID)
	if err != nil {
		return WalletInfo{}, Error.Wrap(err)
	}
	return WalletInfo{
		Address: address,
		Balance: balance,
	}, nil
}

// WalletPayments returns with all the native blockchain payments for a user's wallet.
func (payment Payments) WalletPayments(ctx context.Context) (_ WalletPayments, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := GetUser(ctx)
	if err != nil {
		return WalletPayments{}, Error.Wrap(err)
	}
	address, err := payment.service.depositWallets.Get(ctx, user.ID)
	if err != nil {
		return WalletPayments{}, Error.Wrap(err)
	}

	walletPayments, err := payment.service.depositWallets.Payments(ctx, address, 3000, 0)
	if err != nil {
		return WalletPayments{}, Error.Wrap(err)
	}
	txInfos, err := payment.service.accounts.StorjTokens().ListTransactionInfos(ctx, user.ID)
	if err != nil {
		return WalletPayments{}, Error.Wrap(err)
	}
	txns, err := payment.service.billing.ListSource(ctx, user.ID, billing.StorjScanBonusSource)
	if err != nil {
		return WalletPayments{}, Error.Wrap(err)
	}

	var paymentInfos []PaymentInfo
	for _, walletPayment := range walletPayments {
		source := payment.service.paymentSourceChainIDs[walletPayment.ChainID]
		paymentInfos = append(paymentInfos, PaymentInfo{
			ID:        fmt.Sprintf("%s#%d", walletPayment.Transaction.Hex(), walletPayment.LogIndex),
			Type:      "storjscan",
			Wallet:    walletPayment.To.Hex(),
			Amount:    walletPayment.USDValue,
			Status:    string(walletPayment.Status),
			Link:      payment.BlockExplorerURL(walletPayment.Transaction.Hex(), source),
			Timestamp: walletPayment.Timestamp,
		})
	}
	for _, txInfo := range txInfos {
		paymentInfos = append(paymentInfos, PaymentInfo{
			ID:        txInfo.ID.String(),
			Type:      "coinpayments",
			Wallet:    txInfo.Address,
			Amount:    currency.AmountFromBaseUnits(txInfo.AmountCents, currency.USDollars),
			Received:  currency.AmountFromBaseUnits(txInfo.ReceivedCents, currency.USDollars),
			Status:    txInfo.Status.String(),
			Link:      txInfo.Link,
			Timestamp: txInfo.CreatedAt.UTC(),
		})
	}
	for _, txn := range txns {
		var meta struct {
			ReferenceID string
			LogIndex    int
		}
		err = json.NewDecoder(bytes.NewReader(txn.Metadata)).Decode(&meta)
		if err != nil {
			return WalletPayments{}, Error.Wrap(err)
		}
		paymentInfos = append(paymentInfos, PaymentInfo{
			ID:        strconv.FormatInt(txn.ID, 10),
			Type:      txn.Source,
			Wallet:    address.Hex(),
			Amount:    txn.Amount,
			Status:    string(txn.Status),
			Link:      payment.BlockExplorerURL(meta.ReferenceID, txn.Source),
			Timestamp: txn.Timestamp,
		})
	}

	return WalletPayments{
		Payments: paymentInfos,
	}, nil
}

// WalletPaymentsWithConfirmations returns with all the native blockchain payments (including pending) for a user's wallet.
func (payment Payments) WalletPaymentsWithConfirmations(ctx context.Context) (paymentsWithConfirmations []payments.WalletPaymentWithConfirmations, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := GetUser(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	address, err := payment.service.depositWallets.Get(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	paymentsWithConfirmations, err = payment.service.depositWallets.PaymentsWithConfirmations(ctx, address)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// Purchase makes a purchase of `price` amount with description of `desc` and payment method with id of `token`.
// If a paid invoice with the same description exists, then we assume this is a retried request and don't create and pay
// another invoice.
func (payment Payments) Purchase(ctx context.Context, params *payments.PurchaseParams) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := GetUser(ctx)
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	// Unlikely to happen.
	if params == nil {
		return Error.New("purchase params are empty")
	}

	switch params.Intent {
	case payments.PurchasePackageIntent:
		if !payment.service.config.PricingPackagesEnabled {
			return ErrForbidden.New("pricing packages are not enabled")
		}

		pkg, err := payment.GetPackagePlanByUserAgent(user.UserAgent)
		if err != nil {
			return ErrNotFound.Wrap(err)
		}

		card, err := payment.AddCardByPaymentMethodID(ctx, &params.AddCardParams, true)
		if err != nil {
			return err
		}

		description := string(user.UserAgent) + " package plan"
		err = payment.UpdatePackage(ctx, description, time.Now())
		if err != nil {
			if !ErrAlreadyHasPackage.Has(err) {
				return err
			}
		}

		err = payment.applyCreditFromPaidInvoice(ctx, addCreditFromPaidInvoiceParams{
			User:            user,
			PaymentMethodID: card.ID,
			Price:           pkg.Price,
			Credit:          pkg.Credit,
			Description:     description,
		})
		if err != nil {
			return err
		}
	case payments.PurchaseUpgradedAccountIntent:
		if payment.service.config.UpgradePayUpfrontAmount == 0 {
			return ErrForbidden.New("upgrade to paid account via purchase is not enabled")
		}

		card, err := payment.AddCardByPaymentMethodID(ctx, &params.AddCardParams, false)
		if err != nil {
			return err
		}

		payUpfrontAmount := payment.service.config.UpgradePayUpfrontAmount

		err = payment.applyCreditFromPaidInvoice(ctx, addCreditFromPaidInvoiceParams{
			User:            user,
			PaymentMethodID: card.ID,
			Price:           int64(payUpfrontAmount),
			Credit:          int64(payUpfrontAmount),
			Description:     "Upgrade account - $" + strconv.Itoa(payUpfrontAmount/100) + " credits added to your account balance.",
		})
		if err != nil {
			removeErr := payment.service.accounts.CreditCards().Remove(ctx, user.ID, card.ID, true)
			if removeErr != nil {
				payment.service.log.Warn("failed to remove credit card after failed purchase", zap.Error(removeErr), zap.String("cardID", card.ID), zap.String("userID", user.ID.String()))
			}

			return err
		}
	}

	return nil
}

func (payment Payments) updateCustomerBillingInfo(ctx context.Context, userID uuid.UUID, address *payments.AddAddressParams, tax *payments.AddTaxParams) error {
	if address == nil && tax == nil {
		return nil
	}

	cusID, err := payment.service.store.Users().GetCustomerID(ctx, userID)
	if err != nil {
		return err
	}

	if address != nil {
		if _, err = payment.service.accounts.SaveBillingAddress(ctx, cusID, userID, payments.BillingAddress{
			Name:       address.Name,
			Line1:      address.Line1,
			Line2:      address.Line2,
			City:       address.City,
			PostalCode: address.PostalCode,
			State:      address.State,
			Country: payments.TaxCountry{
				Code: payments.CountryCode(address.Country),
			},
		}); err != nil {
			return err
		}
	}

	if tax != nil {
		if _, err = payment.service.accounts.AddTaxID(ctx, cusID, userID, *tax); err != nil {
			return err
		}
	}

	return nil
}

type addCreditFromPaidInvoiceParams struct {
	User            *User
	PaymentMethodID string
	Price           int64
	Credit          int64
	Description     string
}

func (payment Payments) applyCreditFromPaidInvoice(ctx context.Context, params addCreditFromPaidInvoiceParams) error {
	// Unlikely to happen.
	if params.User == nil {
		return ErrUnauthorized.New("user is not authorized")
	}

	invoices, err := payment.service.accounts.Invoices().List(ctx, params.User.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	var invoiceToPay *payments.Invoice

	for _, inv := range invoices {
		if inv.Description != params.Description {
			continue
		}

		if inv.Status == payments.InvoiceStatusPaid {
			return nil
		}

		if inv.Status == payments.InvoiceStatusDraft {
			_, err := payment.service.accounts.Invoices().Delete(ctx, inv.ID)
			if err != nil {
				return Error.Wrap(err)
			}
		} else if inv.Status == payments.InvoiceStatusOpen {
			invoiceToPay = &inv
		}
	}

	if invoiceToPay == nil {
		invoiceToPay, err = payment.service.accounts.Invoices().Create(ctx, params.User.ID, params.Price, params.Description)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	_, err = payment.service.accounts.Invoices().Pay(ctx, invoiceToPay.ID, params.PaymentMethodID)
	if err != nil {
		return Error.Wrap(err)
	}

	if err = payment.ApplyCredit(ctx, params.Credit, params.Description); err != nil {
		return err
	}

	if params.User.IsFreeOrMember() {
		err = payment.upgradeToPaidTier(ctx, params.User)
		if err != nil {
			return err
		}

		payment.service.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: params.User.Email}},
			&UpgradeToProEmail{LoginURL: payment.service.config.LoginURL},
		)
	}

	return nil
}

// GetPackagePlanByUserAgent returns a package plan by user agent.
func (payment Payments) GetPackagePlanByUserAgent(userAgent []byte) (payments.PackagePlan, error) {
	entries, err := useragent.ParseEntries(userAgent)
	if err != nil {
		return payments.PackagePlan{}, Error.Wrap(err)
	}
	for _, entry := range entries {
		if pkg, ok := payment.service.packagePlans[entry.Product]; ok {
			return pkg, nil
		}
	}
	return payments.PackagePlan{}, Error.New("no matching partner for (%s)", userAgent)
}

// UpdatePackage updates a user's package information unless they already have a package.
func (payment Payments) UpdatePackage(ctx context.Context, packagePlan string, purchaseTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := GetUser(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	dbPackagePlan, dbPurchaseTime, err := payment.service.accounts.GetPackageInfo(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}
	if dbPackagePlan != nil || dbPurchaseTime != nil {
		return ErrAlreadyHasPackage.New("user already has package")
	}

	err = payment.service.accounts.UpdatePackage(ctx, user.ID, &packagePlan, &purchaseTime)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// ApplyCredit applies a credit of `amount` with description of `desc` to the user's balance. `amount` is in cents USD.
// If a credit with `desc` already exists, another one will not be created.
func (payment Payments) ApplyCredit(ctx context.Context, amount int64, desc string) (err error) {
	defer mon.Task()(&ctx)(&err)

	if desc == "" {
		return ErrPurchaseDesc.New("description cannot be empty")
	}
	user, err := GetUser(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	btxs, err := payment.service.accounts.Balances().ListTransactions(ctx, user.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	// check for any previously created transaction with the same description.
	for _, btx := range btxs {
		if btx.Description == desc {
			return nil
		}
	}

	_, err = payment.service.accounts.Balances().ApplyCredit(ctx, user.ID, amount, desc, "")
	if err != nil {
		return Error.Wrap(err)
	}
	return nil
}

// GetProjectUsagePriceModel returns the project usage price model for the partner.
func (payment Payments) GetProjectUsagePriceModel(partner string) (_ *payments.ProjectUsagePriceModel) {
	model := payment.service.accounts.GetProjectUsagePriceModel(partner)
	return &model
}

// GetPartnerPlacementPriceModel returns the product ID and related project usage price model for the project's user agent and placement.
func (payment Payments) GetPartnerPlacementPriceModel(ctx context.Context, projectID uuid.UUID, placement storj.PlacementConstraint) (productID int32, _ payments.ProjectUsagePriceModel, _ error) {
	user, err := GetUser(ctx)
	if err != nil {
		return 0, payments.ProjectUsagePriceModel{}, ErrUnauthorized.Wrap(err)
	}
	isMember, err := payment.service.isProjectMember(ctx, user.ID, projectID)
	if err != nil {
		return 0, payments.ProjectUsagePriceModel{}, ErrUnauthorized.Wrap(err)
	}

	project := isMember.project
	productID, model := payment.service.accounts.GetPartnerPlacementPriceModel(ctx, project.PublicID, string(project.UserAgent), placement)

	return productID, model.ProjectUsagePriceModel, nil
}

func findMembershipByProjectID(memberships []ProjectMember, projectID uuid.UUID) (ProjectMember, bool) {
	for _, membership := range memberships {
		if membership.ProjectID == projectID {
			return membership, true
		}
	}
	return ProjectMember{}, false
}

// GetPagedActiveSessions returns paged active webapp sessions list for given User.
func (s *Service) GetPagedActiveSessions(ctx context.Context, cursor consoleauth.WebappSessionsCursor) (page *consoleauth.WebappSessionsPage, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get active sessions")
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	page, err = s.store.WebappSessions().GetPagedActiveByUserID(ctx, user.ID, s.nowFn(), cursor)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return page, err
}

// InvalidateSession invalidates the session by ID.
func (s *Service) InvalidateSession(ctx context.Context, sessionID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "invalidate session")
	if err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	session, err := s.store.WebappSessions().GetBySessionID(ctx, sessionID)
	if err != nil {
		return Error.Wrap(err)
	}

	if session.UserID != user.ID {
		return ErrUnauthorized.New("session does not belong to the user")
	}

	return Error.Wrap(s.store.WebappSessions().DeleteBySessionID(ctx, session.ID))
}

// DeleteSession removes the session from the database.
func (s *Service) DeleteSession(ctx context.Context, sessionID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(s.store.WebappSessions().DeleteBySessionID(ctx, sessionID))
}

// DeleteAllSessionsByUserIDExcept removes all sessions except the specified session from the database.
func (s *Service) DeleteAllSessionsByUserIDExcept(ctx context.Context, userID uuid.UUID, sessionID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.store.WebappSessions().DeleteAllByUserIDExcept(ctx, userID, sessionID)
	return Error.Wrap(err)
}

// RefreshSession resets the expiration time of the session.
func (s *Service) RefreshSession(ctx context.Context, sessionID uuid.UUID) (expiresAt time.Time, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "refresh session")
	if err != nil {
		return time.Time{}, Error.Wrap(err)
	}

	duration := time.Duration(s.config.Session.InactivityTimerDuration) * time.Second
	settings, err := s.store.Users().GetSettings(ctx, user.ID)
	if err != nil && !errs.Is(err, sql.ErrNoRows) {
		return time.Time{}, Error.Wrap(err)
	}
	if settings != nil && settings.SessionDuration != nil {
		duration = *settings.SessionDuration
	}
	expiresAt = time.Now().Add(duration)

	err = s.store.WebappSessions().UpdateExpiration(ctx, sessionID, expiresAt)
	if err != nil {
		return time.Time{}, err
	}

	return expiresAt, nil
}

// VerifyForgotPasswordCaptcha returns whether the given captcha response for the forgot password page is valid.
// It will return true without error if the captcha handler has not been set.
func (s *Service) VerifyForgotPasswordCaptcha(ctx context.Context, responseToken, userIP string) (valid bool, err error) {
	defer mon.Task()(&ctx)(&err)

	if s.loginCaptchaHandler != nil {
		valid, _, err = s.loginCaptchaHandler.Verify(ctx, responseToken, userIP)
		return valid, ErrCaptcha.Wrap(err)
	}
	return true, nil
}

// GetUserSettings fetches a user's settings. It creates default settings if none exists.
func (s *Service) GetUserSettings(ctx context.Context) (settings *UserSettings, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get user settings")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	settings, err = s.store.Users().GetSettings(ctx, user.ID)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}

		settingsReq := UpsertUserSettingsRequest{}
		// a user may have existed before a corresponding row was created in the user settings table
		// to avoid showing an old user the onboarding flow again, we check to see if the user owns any projects already
		// if so, set the "onboarding start" and "onboarding end" fields to "true"
		projects, err := s.store.Projects().GetOwn(ctx, user.ID)
		if err != nil {
			// we can still proceed with the settings upsert if there is an error retrieving projects, so log and don't return
			s.log.Warn("received error trying to get user's projects", zap.Error(err))
		}
		if len(projects) > 0 {
			t := true
			settingsReq.OnboardingStart = &(t)
			settingsReq.OnboardingEnd = &(t)
		}

		err = s.store.Users().UpsertSettings(ctx, user.ID, settingsReq)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		settings, err = s.store.Users().GetSettings(ctx, user.ID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return settings, nil
}

// SetUserSettings updates a user's settings.
func (s *Service) SetUserSettings(ctx context.Context, request UpsertUserSettingsRequest) (settings *UserSettings, err error) {
	defer mon.Task()(&ctx)(&err)

	fields := []zapcore.Field{}

	if request.OnboardingStart != nil {
		fields = append(fields, zap.Bool("onboardingStart", *request.OnboardingStart))
	}
	if request.OnboardingEnd != nil {
		fields = append(fields, zap.Bool("onboardingEnd", *request.OnboardingEnd))
	}
	if request.OnboardingStep != nil {
		fields = append(fields, zap.String("onboardingStep", *request.OnboardingStep))
	}

	user, err := s.getUserAndAuditLog(ctx, "set user settings", fields...)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = s.store.Users().UpsertSettings(ctx, user.ID, request)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	settings, err = s.store.Users().GetSettings(ctx, user.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return settings, nil
}

// GetUserProjectInvitations returns a user's pending project member invitations.
func (s *Service) GetUserProjectInvitations(ctx context.Context) (_ []ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get project member invitations")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	invites, err := s.store.ProjectInvitations().GetForActiveProjectsByEmail(ctx, user.Email)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var active []ProjectInvitation
	for _, invite := range invites {
		if !s.IsProjectInvitationExpired(&invite) {
			active = append(active, invite)
		}
	}

	return active, nil
}

// ProjectInvitationResponse represents a response to a project member invitation.
type ProjectInvitationResponse int

const (
	// ProjectInvitationDecline represents rejection of a project member invitation.
	ProjectInvitationDecline ProjectInvitationResponse = iota
	// ProjectInvitationAccept represents acceptance of a project member invitation.
	ProjectInvitationAccept
)

// RespondToProjectInvitation handles accepting or declining a user's project member invitation.
// The given project ID may be the internal or public ID.
func (s *Service) RespondToProjectInvitation(ctx context.Context, projectID uuid.UUID, response ProjectInvitationResponse) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "project member invitation response",
		zap.String("projectID", projectID.String()),
		zap.Any("response", response),
	)
	if err != nil {
		return Error.Wrap(err)
	}

	if response != ProjectInvitationAccept && response != ProjectInvitationDecline {
		return ErrValidation.New(projInviteResponseInvalidErrMsg)
	}

	if user.Status == PendingBotVerification {
		return ErrBotUser.New(contactSupportErrMsg)
	}

	proj, err := s.GetProjectNoAuth(ctx, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	if proj.Status != nil && *proj.Status == ProjectDisabled {
		return ErrUnauthorized.New(unauthorizedErrMsg)
	}

	projectID = proj.ID

	// log deletion errors that don't affect the outcome
	deleteWithLog := func() {
		err := s.store.ProjectInvitations().Delete(ctx, projectID, user.Email)
		if err != nil {
			s.log.Warn("error deleting project invitation",
				zap.Error(err),
				zap.String("email", user.Email),
				zap.String("projectID", projectID.String()),
			)
		}
	}

	_, err = s.isProjectMember(ctx, user.ID, projectID)
	if err == nil {
		deleteWithLog()
		if response == ProjectInvitationDecline {
			return ErrAlreadyMember.New(projInviteAlreadyMemberErrMsg)
		}
		return nil
	}

	invite, err := s.store.ProjectInvitations().Get(ctx, projectID, user.Email)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return Error.Wrap(err)
		}
		if response == ProjectInvitationDecline {
			return nil
		}
		return ErrProjectInviteInvalid.New(projInviteInvalidErrMsg)
	}

	if s.IsProjectInvitationExpired(invite) {
		return ErrProjectInviteInvalid.New(projInviteInvalidErrMsg)
	}

	if response == ProjectInvitationDecline {
		return Error.Wrap(s.store.ProjectInvitations().Delete(ctx, projectID, user.Email))
	}

	// check inviter status

	if invite.InviterID != nil {
		inviter, err := s.store.Users().Get(ctx, *invite.InviterID)
		if err != nil {
			if errs.Is(err, sql.ErrNoRows) {
				return ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
			}
			return Error.Wrap(err)
		}
		if inviter.Status != Active {
			return ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
		}

		var userTenant, inviterTenant string
		if user.TenantID != nil {
			userTenant = *user.TenantID
		}
		if inviter.TenantID != nil {
			inviterTenant = *inviter.TenantID
		}
		if userTenant != inviterTenant {
			return ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
		}

		_, err = s.store.ProjectMembers().GetByMemberIDAndProjectID(ctx, *invite.InviterID, invite.ProjectID)
		if err != nil {
			if !errs.Is(err, sql.ErrNoRows) {
				return Error.Wrap(err)
			}
			return ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
		}
	}

	// All the new team members have regular Member role, which can be updated by the project owner later.
	_, err = s.store.ProjectMembers().Insert(ctx, user.ID, projectID, RoleMember)
	if err != nil {
		return Error.Wrap(err)
	}

	deleteWithLog()

	return nil
}

// ProjectInvitationOption represents whether a project invitation request is for
// inviting new members (creating records) or resending existing invitations (updating records).
type ProjectInvitationOption int

const (
	// ProjectInvitationCreate indicates to insert new project member records.
	ProjectInvitationCreate ProjectInvitationOption = iota
	// ProjectInvitationResend indicates to update existing project member records.
	ProjectInvitationResend
)

// ReinviteProjectMembers resends project invitations to the users specified by the given email slice.
// The provided project ID may be the public or internal ID.
func (s *Service) ReinviteProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (invites []ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx,
		"reinvite project members",
		zap.String("projectID", projectID.String()),
		zap.Strings("emails", emails),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return s.inviteProjectMembers(ctx, user, projectID, emails, ProjectInvitationResend)
}

// InviteNewProjectMember invites a user by email to the project specified by the given ID,
// which may be its public or internal ID.
func (s *Service) InviteNewProjectMember(ctx context.Context, projectID uuid.UUID, email string) (invite *ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx,
		"invite project member",
		zap.String("projectID", projectID.String()),
		zap.String("invitedEmail", email),
	)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	invites, err := s.inviteProjectMembers(ctx, user, projectID, []string{email}, ProjectInvitationCreate)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &invites[0], nil
}

// inviteProjectMembers invites users by email to the project specified by the given ID,
// which may be its public or internal ID.
func (s *Service) inviteProjectMembers(ctx context.Context, sender *User, projectID uuid.UUID, emails []string, opt ProjectInvitationOption) (invites []ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	isMember, err := s.isProjectMember(ctx, sender.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	if isMember.membership.Role != RoleAdmin {
		return nil, ErrForbidden.New("only project Owner or Admin can invite other members")
	}

	projectID = isMember.project.ID

	var users []*User
	var newUserEmails []string
	var unverifiedUsers []User
	for _, email := range emails {
		invite, err := s.store.ProjectInvitations().Get(ctx, projectID, email)
		if err != nil && !errs.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}

		if invite != nil {
			// If we should only insert new records, a preexisting record is an issue
			if opt == ProjectInvitationCreate {
				return nil, ErrAlreadyInvited.New(projInviteExistsErrMsg, email)
			}
			if !s.IsProjectInvitationExpired(invite) {
				return nil, ErrAlreadyInvited.New(activeProjInviteExistsErrMsg, email)
			}
		} else if opt == ProjectInvitationResend {
			// If we should only update existing records, an absence of records is an issue
			return nil, ErrProjectInviteInvalid.New(projInviteDoesntExistErrMsg, email)
		}

		invitedUser, unverified, err := s.store.Users().GetByEmailAndTenantWithUnverified(ctx, email, sender.TenantID)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		if invitedUser != nil {
			_, err = s.isProjectMember(ctx, invitedUser.ID, projectID)
			if err != nil && !ErrNoMembership.Has(err) {
				return nil, Error.Wrap(err)
			} else if err == nil {
				return nil, ErrAlreadyMember.New("%s is already a member", email)
			}
			users = append(users, invitedUser)
		} else if len(unverified) > 0 {
			oldest := unverified[0]
			for _, u := range unverified {
				if u.CreatedAt.Before(oldest.CreatedAt) {
					oldest = u
				}
			}

			if oldest.Status != Inactive {
				return nil, errs.New("there was an error inviting user %s. Please contact support", email)
			}

			unverifiedUsers = append(unverifiedUsers, oldest)
		} else if s.config.UnregisteredInviteEmailsEnabled {
			newUserEmails = append(newUserEmails, email)
		}
	}

	inviteTokens := make(map[string]string)
	// add project invites in transaction scope
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		for _, email := range emails {
			invite, err := tx.ProjectInvitations().Upsert(ctx, &ProjectInvitation{
				ProjectID: projectID,
				Email:     email,
				InviterID: &sender.ID,
			})
			if err != nil {
				return err
			}

			var isUnverified bool
			for _, u := range unverifiedUsers {
				if email == u.Email {
					isUnverified = true
					invites = append(invites, *invite)
					break
				}
			}
			if isUnverified {
				continue
			}

			token, err := s.CreateInviteToken(ctx, isMember.project.PublicID, email, invite.CreatedAt)
			if err != nil {
				return err
			}
			inviteTokens[email] = token
			invites = append(invites, *invite)
		}
		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	baseLink := s.satelliteAddress + "/invited"
	for _, invited := range users {
		inviteLink := fmt.Sprintf("%s?invite=%s", baseLink, inviteTokens[invited.Email])

		userName := invited.ShortName
		if userName == "" {
			userName = invited.FullName
		}
		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: invited.Email, Name: userName}},
			&ExistingUserProjectInvitationEmail{
				InviterEmail: sender.Email,
				Region:       s.satelliteName,
				SignInLink:   inviteLink,
			},
		)
	}
	for _, u := range unverifiedUsers {
		token, err := s.GenerateActivationToken(ctx, u.ID, u.Email)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		activationLink := fmt.Sprintf("%s/activation?token=%s", s.satelliteAddress, token)
		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: u.Email}},
			&UnverifiedUserProjectInvitationEmail{
				InviterEmail:   sender.Email,
				Region:         s.satelliteName,
				ActivationLink: activationLink,
			},
		)
	}

	for _, email := range newUserEmails {
		inviteLink := fmt.Sprintf("%s?invite=%s", baseLink, inviteTokens[email])
		s.mailService.SendRenderedAsync(
			ctx,
			[]post.Address{{Address: email}},
			&NewUserProjectInvitationEmail{
				InviterEmail: sender.Email,
				Region:       s.satelliteName,
				SignUpLink:   inviteLink,
			},
		)
	}

	return invites, nil
}

// IsProjectInvitationExpired returns whether the project member invitation has expired.
func (s *Service) IsProjectInvitationExpired(invite *ProjectInvitation) bool {
	return time.Now().After(invite.CreatedAt.Add(s.config.ProjectInvitationExpiration))
}

// GetInvitesByEmail returns project invites by email.
func (s *Service) GetInvitesByEmail(ctx context.Context, email string) (invites []ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	return s.store.ProjectInvitations().GetByEmail(ctx, email)
}

// GetInviteByToken returns a project invite given an invite token.
func (s *Service) GetInviteByToken(ctx context.Context, token string) (invite *ProjectInvitation, err error) {
	defer mon.Task()(&ctx)(&err)

	publicProjectID, email, err := s.ParseInviteToken(ctx, token)
	if err != nil {
		return nil, ErrProjectInviteInvalid.Wrap(err)
	}

	project, err := s.store.Projects().GetByPublicID(ctx, publicProjectID)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}
		return nil, ErrProjectInviteInvalid.New(projInviteInvalidErrMsg)
	}

	invite, err = s.store.ProjectInvitations().Get(ctx, project.ID, email)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return nil, Error.Wrap(err)
		}
		return nil, ErrProjectInviteInvalid.New(projInviteInvalidErrMsg)
	}
	if s.IsProjectInvitationExpired(invite) {
		return nil, ErrProjectInviteInvalid.New(projInviteInvalidErrMsg)
	}

	if invite.InviterID != nil {
		inviter, err := s.store.Users().Get(ctx, *invite.InviterID)
		if err != nil {
			if errs.Is(err, sql.ErrNoRows) {
				return nil, ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
			}
			return nil, Error.Wrap(err)
		}
		if inviter.Status != Active {
			return nil, ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
		}

		_, err = s.store.ProjectMembers().GetByMemberIDAndProjectID(ctx, *invite.InviterID, invite.ProjectID)
		if err != nil {
			if errs.Is(err, sql.ErrNoRows) {
				return nil, ErrProjectInviteInvalid.New(projInviterInvalidErrMsg)
			}
			return nil, Error.Wrap(err)
		}
	}

	return invite, nil
}

// GetInviteLink returns a link for project invites.
func (s *Service) GetInviteLink(ctx context.Context, publicProjectID uuid.UUID, email string) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "get invite link", zap.String("projectID", publicProjectID.String()), zap.String("email", email))
	if err != nil {
		return "", Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, user.ID, publicProjectID)
	if err != nil {
		return "", ErrUnauthorized.Wrap(err)
	}

	if isMember.membership.Role != RoleAdmin {
		return "", ErrForbidden.New("only project Owner or Admin can get an invite link")
	}

	invite, err := s.store.ProjectInvitations().Get(ctx, isMember.project.ID, email)
	if err != nil {
		if !errs.Is(err, sql.ErrNoRows) {
			return "", Error.Wrap(err)
		}
		return "", ErrProjectInviteInvalid.New(projInviteInvalidErrMsg)
	}

	token, err := s.CreateInviteToken(ctx, publicProjectID, email, invite.CreatedAt)
	if err != nil {
		return "", Error.Wrap(err)
	}

	return fmt.Sprintf("%s/invited?invite=%s", s.satelliteAddress, token), nil
}

// CreateInviteToken creates a token for project invite links.
// Internal use only, since it doesn't check if the project is valid or the user is a member of the project.
func (s *Service) CreateInviteToken(ctx context.Context, publicProjectID uuid.UUID, email string, inviteDate time.Time) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)

	linkClaims := consoleauth.Claims{
		ID:         publicProjectID,
		Email:      email,
		Expiration: inviteDate.Add(s.config.ProjectInvitationExpiration),
	}

	claimJson, err := linkClaims.JSON()
	if err != nil {
		return "", err
	}

	token := consoleauth.Token{Payload: claimJson}
	signature, err := s.tokens.SignToken(token)
	if err != nil {
		return "", err
	}
	token.Signature = signature

	return token.String(), nil
}

// ParseInviteToken parses a token from project invite links.
func (s *Service) ParseInviteToken(ctx context.Context, token string) (publicID uuid.UUID, email string, err error) {
	defer mon.Task()(&ctx)(&err)

	parsedToken, err := consoleauth.FromBase64URLString(token)
	valid, err := s.tokens.ValidateToken(parsedToken)
	if err != nil {
		return uuid.UUID{}, "", err
	}
	if !valid {
		return uuid.UUID{}, "", ErrTokenInvalid.New("incorrect signature")
	}

	claims, err := consoleauth.FromJSON(parsedToken.Payload)
	if err != nil {
		return uuid.UUID{}, "", ErrTokenInvalid.New("JSON decoder: %w", err)
	}

	if time.Now().After(claims.Expiration) {
		return uuid.UUID{}, "", ErrTokenExpiration.New("invite token expired")
	}

	return claims.ID, claims.Email, nil
}

// TestSetNow allows tests to have the Service act as if the current time is whatever they want.
func (s *Service) TestSetNow(now func() time.Time) {
	s.nowFn = now
}

// TestSetAuditableAPIKeyProjects is used in tests to set the list of projects that can be audited via API keys.
func (s *Service) TestSetAuditableAPIKeyProjects(list map[string]struct{}) {
	s.auditableAPIKeyProjects = list
}

// TestToggleSatelliteManagedEncryption toggles the satellite managed encryption config for tests.
func (s *Service) TestToggleSatelliteManagedEncryption(b bool) {
	s.config.SatelliteManagedEncryptionEnabled = b
}

// TestToggleManagedEncryptionPathEncryption toggles whether managed encryption projects should have
// path encryption in tests.
func (s *Service) TestToggleManagedEncryptionPathEncryption(b bool) {
	s.config.ManagedEncryption.PathEncryptionEnabled = b
}

// TestToggleSsoEnabled is used in tests to toggle SSO.
func (s *Service) TestToggleSsoEnabled(enabled bool, ssoService *sso.Service) {
	s.ssoEnabled = enabled
	s.ssoService = ssoService
}

// TestSetNewUsageReportEnabled is used in tests to toggle the new usage report.
func (s *Service) TestSetNewUsageReportEnabled(enabled bool) {
	s.config.NewDetailedUsageReportEnabled = enabled
}

// TestMinimumChargeConfig is used in tests to call TestSetMinimumChargeConfig.
type TestMinimumChargeConfig struct {
	Amount        int64
	EffectiveDate *time.Time
}

// TestSetMinimumChargeConfig is used in tests to set the minimum charge config.
func (s *Service) TestSetMinimumChargeConfig(cfg TestMinimumChargeConfig) {
	s.minimumChargeAmount = cfg.Amount
	s.minimumChargeDate = cfg.EffectiveDate
}

// ValidateFreeFormFieldLengths checks if any of the given values
// exceeds the maximum length.
func (s *Service) ValidateFreeFormFieldLengths(values ...*string) error {
	for _, value := range values {
		if value != nil && utf8.RuneCountInString(*value) > s.config.MaxNameCharacters {
			return ErrValidation.New("field length exceeds maximum length %d", s.config.MaxNameCharacters)
		}
	}
	return nil
}

// ValidateLongFormInputLengths checks if any of the given values
// exceeds the maximum length for long form fields.
func (s *Service) ValidateLongFormInputLengths(values ...*string) error {
	for _, value := range values {
		if value != nil && utf8.RuneCountInString(*value) > s.config.MaxLongFormFieldCharacters {
			return ErrValidation.New("field length exceeds maximum length %d", s.config.MaxLongFormFieldCharacters)
		}
	}
	return nil
}
