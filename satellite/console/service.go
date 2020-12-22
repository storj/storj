// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stripe/stripe-go"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/payments"
	"storj.io/storj/satellite/rewards"
)

var mon = monkit.Package()

const (
	// maxLimit specifies the limit for all paged queries.
	maxLimit            = 50
	tokenExpirationTime = 24 * time.Hour

	// TestPasswordCost is the hashing complexity to use for testing.
	TestPasswordCost = bcrypt.MinCost
)

// Error messages.
const (
	unauthorizedErrMsg                   = "You are not authorized to perform this action"
	emailUsedErrMsg                      = "This email is already in use, try another"
	passwordRecoveryTokenIsExpiredErrMsg = "Your password recovery link has expired, please request another one"
	credentialsErrMsg                    = "Your email or password was incorrect, please try again"
	passwordIncorrectErrMsg              = "Your password needs at least %d characters long"
	projectOwnerDeletionForbiddenErrMsg  = "%s is a project owner and can not be deleted"
	apiKeyWithNameExistsErrMsg           = "An API Key with this name already exists in this project, please use a different name"
	teamMemberDoesNotExistErrMsg         = `There is no account on this Satellite for the user(s) you have entered.
									     Please add team members with active accounts`

	usedRegTokenErrMsg = "This registration token has already been used"
	projLimitErrMsg    = "Sorry, project creation is limited for your account. Please contact support!"
)

var (
	// Error describes internal console error.
	Error = errs.Class("service error")

	// ErrNoMembership is error type of not belonging to a specific project.
	ErrNoMembership = errs.Class("no membership error")

	// ErrTokenExpiration is error type of token reached expiration time.
	ErrTokenExpiration = errs.Class("token expiration error")

	// ErrProjLimit is error type of project limit.
	ErrProjLimit = errs.Class("project limit error")

	// ErrUsage is error type of project usage.
	ErrUsage = errs.Class("project usage error")

	// ErrEmailUsed is error type that occurs on repeating auth attempts with email.
	ErrEmailUsed = errs.Class("email used")
)

// Service is handling accounts related logic.
//
// architecture: Service
type Service struct {
	Signer

	log, auditLogger  *zap.Logger
	store             DB
	projectAccounting accounting.ProjectAccounting
	projectUsage      *accounting.Service
	buckets           Buckets
	rewards           rewards.DB
	partners          *rewards.PartnersService
	accounts          payments.Accounts

	config Config

	minCoinPayment int64
}

// Config keeps track of core console service configuration parameters.
type Config struct {
	PasswordCost            int  `help:"password hashing cost (0=automatic)" internal:"true" default:"0"`
	OpenRegistrationEnabled bool `help:"enable open registration" default:"false"`
	DefaultProjectLimit     int  `help:"default project limits for users" default:"10"`
}

// PaymentsService separates all payment related functionality.
type PaymentsService struct {
	service *Service
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, signer Signer, store DB, projectAccounting accounting.ProjectAccounting, projectUsage *accounting.Service, buckets Buckets, rewards rewards.DB, partners *rewards.PartnersService, accounts payments.Accounts, config Config, minCoinPayment int64) (*Service, error) {
	if signer == nil {
		return nil, errs.New("signer can't be nil")
	}
	if store == nil {
		return nil, errs.New("store can't be nil")
	}
	if log == nil {
		return nil, errs.New("log can't be nil")
	}
	if config.PasswordCost == 0 {
		config.PasswordCost = bcrypt.DefaultCost
	}

	return &Service{
		log:               log,
		auditLogger:       log.Named("auditlog"),
		Signer:            signer,
		store:             store,
		projectAccounting: projectAccounting,
		projectUsage:      projectUsage,
		buckets:           buckets,
		rewards:           rewards,
		partners:          partners,
		accounts:          accounts,
		config:            config,
		minCoinPayment:    minCoinPayment,
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
		make([]zap.Field, 0, len(extra)+5),
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
	fields = append(fields, fields...)
	s.auditLogger.Info("console activity", fields...)
}

func (s *Service) getAuthAndAuditLog(ctx context.Context, operation string, extra ...zap.Field) (Authorization, error) {
	auth, err := GetAuth(ctx)
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
		return Authorization{}, err
	}
	s.auditLog(ctx, operation, &auth.User.ID, auth.User.Email, extra...)
	return auth, nil
}

// Payments separates all payment related functionality.
func (s *Service) Payments() PaymentsService {
	return PaymentsService{service: s}
}

// SetupAccount creates payment account for authorized user.
func (paymentService PaymentsService) SetupAccount(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "setup payment account")
	if err != nil {
		return Error.Wrap(err)
	}

	return paymentService.service.accounts.Setup(ctx, auth.User.ID, auth.User.Email)
}

// AccountBalance return account balance.
func (paymentService PaymentsService) AccountBalance(ctx context.Context) (balance payments.Balance, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "get account balance")
	if err != nil {
		return payments.Balance{}, Error.Wrap(err)
	}

	return paymentService.service.accounts.Balance(ctx, auth.User.ID)
}

// AddCreditCard is used to save new credit card and attach it to payment account.
func (paymentService PaymentsService) AddCreditCard(ctx context.Context, creditCardToken string) (err error) {
	defer mon.Task()(&ctx, creditCardToken)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "add credit card")
	if err != nil {
		return Error.Wrap(err)
	}

	err = paymentService.service.accounts.CreditCards().Add(ctx, auth.User.ID, creditCardToken)
	if err != nil {
		return Error.Wrap(err)
	}

	if !paymentService.service.accounts.PaywallEnabled(auth.User.ID) {
		return nil
	}

	// TODO: check if this is the right place
	err = paymentService.AddPromotionalCoupon(ctx, auth.User.ID)
	if err != nil {
		paymentService.service.log.Warn(fmt.Sprintf("could not add promotional coupon for user %s", auth.User.ID.String()), zap.Error(err))
	}

	return nil
}

// MakeCreditCardDefault makes a credit card default payment method.
func (paymentService PaymentsService) MakeCreditCardDefault(ctx context.Context, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "make credit card default")
	if err != nil {
		return Error.Wrap(err)
	}

	return paymentService.service.accounts.CreditCards().MakeDefault(ctx, auth.User.ID, cardID)
}

// ProjectsCharges returns how much money current user will be charged for each project which he owns.
func (paymentService PaymentsService) ProjectsCharges(ctx context.Context, since, before time.Time) (_ []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "project charges")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return paymentService.service.accounts.ProjectCharges(ctx, auth.User.ID, since, before)
}

// ListCreditCards returns a list of credit cards for a given payment account.
func (paymentService PaymentsService) ListCreditCards(ctx context.Context) (_ []payments.CreditCard, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "list credit cards")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return paymentService.service.accounts.CreditCards().List(ctx, auth.User.ID)
}

// RemoveCreditCard is used to detach a credit card from payment account.
func (paymentService PaymentsService) RemoveCreditCard(ctx context.Context, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "remove credit card")
	if err != nil {
		return Error.Wrap(err)
	}

	return paymentService.service.accounts.CreditCards().Remove(ctx, auth.User.ID, cardID)
}

// BillingHistory returns a list of billing history items for payment account.
func (paymentService PaymentsService) BillingHistory(ctx context.Context) (billingHistory []*BillingHistoryItem, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "get billing history")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	invoices, err := paymentService.service.accounts.Invoices().List(ctx, auth.User.ID)
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

	txsInfos, err := paymentService.service.accounts.StorjTokens().ListTransactionInfos(ctx, auth.User.ID)
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

	charges, err := paymentService.service.accounts.Charges(ctx, auth.User.ID)
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

	coupons, err := paymentService.service.accounts.Coupons().ListByUserID(ctx, auth.User.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	for _, coupon := range coupons {
		alreadyUsed, err := paymentService.service.accounts.Coupons().TotalUsage(ctx, coupon.ID)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		remaining := coupon.Amount - alreadyUsed
		if coupon.Status == payments.CouponExpired {
			remaining = 0
		}

		var couponStatus string

		switch coupon.Status {
		case 0:
			couponStatus = "Active"
		case 1:
			couponStatus = "Used"
		default:
			couponStatus = "Expired"
		}

		billingHistory = append(billingHistory,
			&BillingHistoryItem{
				ID:          coupon.ID.String(),
				Description: coupon.Description,
				Amount:      coupon.Amount,
				Remaining:   remaining,
				Status:      couponStatus,
				Link:        "",
				Start:       coupon.Created,
				End:         coupon.ExpirationDate(),
				Type:        Coupon,
			},
		)
	}

	bonuses, err := paymentService.service.accounts.StorjTokens().ListDepositBonuses(ctx, auth.User.ID)
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

// TokenDeposit creates new deposit transaction for adding STORJ tokens to account balance.
func (paymentService PaymentsService) TokenDeposit(ctx context.Context, amount int64) (_ *payments.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "token deposit")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	tx, err := paymentService.service.accounts.StorjTokens().Deposit(ctx, auth.User.ID, amount)
	return tx, Error.Wrap(err)
}

// checkOutstandingInvoice returns if the payment account has any unpaid/outstanding invoices or/and invoice items.
func (paymentService PaymentsService) checkOutstandingInvoice(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := paymentService.service.getAuthAndAuditLog(ctx, "get outstanding invoices")
	if err != nil {
		return err
	}

	invoices, err := paymentService.service.accounts.Invoices().List(ctx, auth.User.ID)
	if err != nil {
		return err
	}
	if len(invoices) > 0 {
		for _, invoice := range invoices {
			if invoice.Status != string(stripe.InvoiceStatusPaid) {
				return ErrUsage.New("user has unpaid/pending invoices")
			}
		}
	}

	hasItems, err := paymentService.service.accounts.Invoices().CheckPendingItems(ctx, auth.User.ID)
	if err != nil {
		return err
	}
	if hasItems {
		return ErrUsage.New("user has pending invoice items")
	}
	return nil
}

// checkProjectInvoicingStatus returns if for the given project there are outstanding project records and/or usage
// which have not been applied/invoiced yet (meaning sent over to stripe).
func (paymentService PaymentsService) checkProjectInvoicingStatus(ctx context.Context, projectID uuid.UUID) (unpaidUsage bool, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = paymentService.service.getAuthAndAuditLog(ctx, "project charges")
	if err != nil {
		return false, Error.Wrap(err)
	}

	return paymentService.service.accounts.CheckProjectInvoicingStatus(ctx, projectID)
}

// PopulatePromotionalCoupons is used to populate promotional coupons through all active users who already have
// a project, payment method and do not have a promotional coupon yet.
// And updates project limits to selected size.
func (paymentService PaymentsService) PopulatePromotionalCoupons(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return Error.Wrap(paymentService.service.accounts.Coupons().PopulatePromotionalCoupons(ctx, 2, 5500, memory.TB))
}

// AddPromotionalCoupon creates new coupon for specified user.
func (paymentService PaymentsService) AddPromotionalCoupon(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx, userID)(&err)

	if paymentService.service.accounts.PaywallEnabled(userID) {
		cards, err := paymentService.ListCreditCards(ctx)
		if err != nil {
			return Error.Wrap(err)
		}
		if len(cards) == 0 {
			return Error.New("user don't have a payment method")
		}
	}

	return paymentService.service.accounts.Coupons().AddPromotionalCoupon(ctx, userID)
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

// CreateUser gets password hash value and creates new inactive User.
func (s *Service) CreateUser(ctx context.Context, user CreateUser, tokenSecret RegistrationSecret, refUserID string) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)
	if err := user.IsValid(); err != nil {
		return nil, Error.Wrap(err)
	}

	offerType := rewards.FreeCredit
	if user.PartnerID != "" {
		offerType = rewards.Partner
	} else if refUserID != "" {
		offerType = rewards.Referral
	}

	// TODO: Create a current offer cache to replace database call
	offers, err := s.rewards.GetActiveOffersByType(ctx, offerType)
	if err != nil && !rewards.ErrOfferNotExist.Has(err) {
		s.log.Error("internal error", zap.Error(err))
		return nil, Error.Wrap(err)
	}

	currentReward, err := s.partners.GetActiveOffer(ctx, offers, offerType, user.PartnerID)
	if err != nil && !rewards.ErrOfferNotExist.Has(err) {
		s.log.Error("internal error", zap.Error(err))
		return nil, Error.Wrap(err)
	}

	registrationToken, err := s.checkRegistrationSecret(ctx, tokenSecret)
	if err != nil {
		return nil, err
	}

	u, err = s.store.Users().GetByEmail(ctx, user.Email)
	if err == nil {
		return nil, ErrEmailUsed.New(emailUsedErrMsg)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, Error.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), s.config.PasswordCost)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// store data
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		userID, err := uuid.New()
		if err != nil {
			return Error.Wrap(err)
		}

		newUser := &User{
			ID:           userID,
			Email:        user.Email,
			FullName:     user.FullName,
			ShortName:    user.ShortName,
			PasswordHash: hash,
			Status:       Inactive,
		}
		if user.PartnerID != "" {
			newUser.PartnerID, err = uuid.FromString(user.PartnerID)
			if err != nil {
				return Error.Wrap(err)
			}
		}

		if registrationToken != nil {
			newUser.ProjectLimit = registrationToken.ProjectLimit
		}

		u, err = tx.Users().Insert(ctx,
			newUser,
		)
		if err != nil {
			return Error.Wrap(err)
		}

		if registrationToken != nil {
			err = tx.RegistrationTokens().UpdateOwner(ctx, registrationToken.Secret, u.ID)
			if err != nil {
				return Error.Wrap(err)
			}
		}

		if currentReward != nil {
			_ = currentReward
			// ToDo: NB: Uncomment this block when UserCredits().Create is cockroach compatible
			// var refID *uuid.UUID
			// if refUserID != "" {
			// 	refID, err = uuid.FromString(refUserID)
			// 	if err != nil {
			// 		return Error.Wrap(err)
			// 	}
			// }
			// newCredit, err := NewCredit(currentReward, Invitee, u.ID, refID)
			// if err != nil {
			// 	return err
			// }
			// err = tx.UserCredits().Create(ctx, *newCredit)
			// if err != nil {
			// 	return err
			// }
		}

		return nil
	})

	if err != nil {
		return nil, Error.Wrap(err)
	}

	s.auditLog(ctx, "create user", nil, user.Email)

	return u, nil
}

// GenerateActivationToken - is a method for generating activation token.
func (s *Service) GenerateActivationToken(ctx context.Context, id uuid.UUID, email string) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	// TODO: activation token should differ from auth token
	claims := &consoleauth.Claims{
		ID:         id,
		Email:      email,
		Expiration: time.Now().Add(time.Hour * 24),
	}

	return s.createToken(ctx, claims)
}

// GeneratePasswordRecoveryToken - is a method for generating password recovery token.
func (s *Service) GeneratePasswordRecoveryToken(ctx context.Context, id uuid.UUID) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	resetPasswordToken, err := s.store.ResetPasswordTokens().GetByOwnerID(ctx, id)
	if err == nil {
		err := s.store.ResetPasswordTokens().Delete(ctx, resetPasswordToken.Secret)
		if err != nil {
			return "", Error.Wrap(err)
		}
	}

	resetPasswordToken, err = s.store.ResetPasswordTokens().Create(ctx, id)
	if err != nil {
		return "", Error.Wrap(err)
	}

	s.auditLog(ctx, "generate password recovery token", &id, "")

	return resetPasswordToken.Secret.String(), nil
}

// ActivateAccount - is a method for activating user account after registration.
func (s *Service) ActivateAccount(ctx context.Context, activationToken string) (err error) {
	defer mon.Task()(&ctx)(&err)

	token, err := consoleauth.FromBase64URLString(activationToken)
	if err != nil {
		return Error.Wrap(err)
	}

	claims, err := s.authenticate(ctx, token)
	if err != nil {
		return err
	}

	_, err = s.store.Users().GetByEmail(ctx, claims.Email)
	if err == nil {
		return ErrEmailUsed.New(emailUsedErrMsg)
	}

	user, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	now := time.Now()

	if now.After(user.CreatedAt.Add(tokenExpirationTime)) {
		return ErrTokenExpiration.Wrap(err)
	}

	user.Status = Active
	err = s.store.Users().Update(ctx, user)
	if err != nil {
		return Error.Wrap(err)
	}
	s.auditLog(ctx, "activate account", &user.ID, user.Email)

	err = s.store.UserCredits().UpdateEarnedCredits(ctx, user.ID)
	if err != nil && !NoCreditForUpdateErr.Has(err) {
		return Error.Wrap(err)
	}

	if s.accounts.PaywallEnabled(user.ID) {
		return nil
	}

	// TODO: check if this is the right place
	err = s.accounts.Coupons().AddPromotionalCoupon(ctx, user.ID)
	if err != nil {
		s.log.Debug(fmt.Sprintf("could not add promotional coupon for user %s", user.ID.String()), zap.Error(Error.Wrap(err)))
	}

	return nil
}

// ResetPassword - is a method for resetting user password.
func (s *Service) ResetPassword(ctx context.Context, resetPasswordToken, password string) (err error) {
	defer mon.Task()(&ctx)(&err)

	secret, err := ResetPasswordSecretFromBase64(resetPasswordToken)
	if err != nil {
		return Error.Wrap(err)
	}
	token, err := s.store.ResetPasswordTokens().GetBySecret(ctx, secret)
	if err != nil {
		return Error.Wrap(err)
	}

	user, err := s.store.Users().Get(ctx, *token.OwnerID)
	if err != nil {
		return Error.Wrap(err)
	}

	if err := ValidatePassword(password); err != nil {
		return Error.Wrap(err)
	}

	if time.Since(token.CreatedAt) > tokenExpirationTime {
		return ErrTokenExpiration.New(passwordRecoveryTokenIsExpiredErrMsg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.config.PasswordCost)
	if err != nil {
		return Error.Wrap(err)
	}

	user.PasswordHash = hash

	err = s.store.Users().Update(ctx, user)
	if err != nil {
		return Error.Wrap(err)
	}
	s.auditLog(ctx, "password reset", &user.ID, user.Email)

	if err = s.store.ResetPasswordTokens().Delete(ctx, token.Secret); err != nil {
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

// Token authenticates User by credentials and returns auth token.
func (s *Service) Token(ctx context.Context, email, password string) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.store.Users().GetByEmail(ctx, email)
	if err != nil {
		return "", ErrUnauthorized.New(credentialsErrMsg)
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password))
	if err != nil {
		return "", ErrUnauthorized.New(credentialsErrMsg)
	}

	claims := consoleauth.Claims{
		ID:         user.ID,
		Expiration: time.Now().Add(tokenExpirationTime),
	}

	token, err = s.createToken(ctx, &claims)
	if err != nil {
		return "", err
	}
	s.auditLog(ctx, "login", &user.ID, user.Email)

	return token, nil
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

// GetUserByEmail returns User by email.
func (s *Service) GetUserByEmail(ctx context.Context, email string) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := s.store.Users().GetByEmail(ctx, email)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// UpdateAccount updates User.
func (s *Service) UpdateAccount(ctx context.Context, fullName string, shortName string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "update account")
	if err != nil {
		return Error.Wrap(err)
	}

	// validate fullName
	err = ValidateFullName(fullName)
	if err != nil {
		return ErrValidation.Wrap(err)
	}

	err = s.store.Users().Update(ctx, &User{
		ID:           auth.User.ID,
		FullName:     fullName,
		ShortName:    shortName,
		Email:        auth.User.Email,
		PasswordHash: nil,
		Status:       auth.User.Status,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// ChangeEmail updates email for a given user.
func (s *Service) ChangeEmail(ctx context.Context, newEmail string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "change email")
	if err != nil {
		return Error.Wrap(err)
	}

	if _, err := mail.ParseAddress(newEmail); err != nil {
		return ErrValidation.Wrap(err)
	}

	_, err = s.store.Users().GetByEmail(ctx, newEmail)
	if err == nil {
		return ErrEmailUsed.New(emailUsedErrMsg)
	}

	auth.User.Email = newEmail
	err = s.store.Users().Update(ctx, &auth.User)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// ChangePassword updates password for a given user.
func (s *Service) ChangePassword(ctx context.Context, pass, newPass string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "change password")
	if err != nil {
		return Error.Wrap(err)
	}

	err = bcrypt.CompareHashAndPassword(auth.User.PasswordHash, []byte(pass))
	if err != nil {
		return ErrUnauthorized.New(credentialsErrMsg)
	}

	if err := ValidatePassword(newPass); err != nil {
		return ErrValidation.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), s.config.PasswordCost)
	if err != nil {
		return Error.Wrap(err)
	}

	auth.User.PasswordHash = hash
	err = s.store.Users().Update(ctx, &auth.User)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// DeleteAccount deletes User.
func (s *Service) DeleteAccount(ctx context.Context, password string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "delete account")
	if err != nil {
		return Error.Wrap(err)
	}

	err = bcrypt.CompareHashAndPassword(auth.User.PasswordHash, []byte(password))
	if err != nil {
		return ErrUnauthorized.New(credentialsErrMsg)
	}

	err = s.Payments().checkOutstandingInvoice(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.store.Users().Delete(ctx, auth.User.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetProject is a method for querying project by id.
func (s *Service) GetProject(ctx context.Context, projectID uuid.UUID) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "get project", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if _, err = s.isProjectMember(ctx, auth.User.ID, projectID); err != nil {
		return nil, Error.Wrap(err)
	}

	p, err = s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// GetUsersProjects is a method for querying all projects.
func (s *Service) GetUsersProjects(ctx context.Context) (ps []Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "get users projects")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	ps, err = s.store.Projects().GetByUserID(ctx, auth.User.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// GetCurrentRewardByType is a method for querying current active reward offer based on its type.
func (s *Service) GetCurrentRewardByType(ctx context.Context, offerType rewards.OfferType) (offer *rewards.Offer, err error) {
	defer mon.Task()(&ctx)(&err)

	offers, err := s.rewards.GetActiveOffersByType(ctx, offerType)
	if err != nil {
		s.log.Error("internal error", zap.Error(err))
		return nil, Error.Wrap(err)
	}

	result, err := s.partners.GetActiveOffer(ctx, offers, offerType, "")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// GetUserCreditUsage is a method for querying users' credit information up until now.
func (s *Service) GetUserCreditUsage(ctx context.Context) (usage *UserCreditUsage, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "get credit card usage")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	usage, err = s.store.UserCredits().GetCreditUsage(ctx, auth.User.ID, time.Now().UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return usage, nil
}

// CreateProject is a method for creating new project.
func (s *Service) CreateProject(ctx context.Context, projectInfo ProjectInfo) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "create project")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = s.checkProjectLimit(ctx, auth.User.ID)
	if err != nil {
		return nil, ErrProjLimit.Wrap(err)
	}

	if s.accounts.PaywallEnabled(auth.User.ID) {
		cards, err := s.accounts.CreditCards().List(ctx, auth.User.ID)
		if err != nil {
			s.log.Debug(fmt.Sprintf("could not list credit cards for user %s", auth.User.ID.String()), zap.Error(Error.Wrap(err)))
			return nil, Error.Wrap(err)
		}

		balance, err := s.accounts.Balance(ctx, auth.User.ID)
		if err != nil {
			s.log.Debug(fmt.Sprintf("could not get balance for user %s", auth.User.ID.String()), zap.Error(Error.Wrap(err)))
			return nil, Error.Wrap(err)
		}

		coupons, err := s.accounts.Coupons().ListByUserID(ctx, auth.User.ID)
		if err != nil {
			s.log.Debug(fmt.Sprintf("could not list coupons for user %s", auth.User.ID.String()), zap.Error(Error.Wrap(err)))
			return nil, Error.Wrap(err)
		}

		if len(cards) == 0 && balance.Coins < s.minCoinPayment && len(coupons) == 0 {
			err = errs.New("no valid payment methods found")
			s.log.Debug(fmt.Sprintf("could not create project for user %s", auth.User.ID.String()), zap.Error(Error.Wrap(err)))
			return nil, Error.Wrap(err)
		}
	}

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		p, err = tx.Projects().Insert(ctx,
			&Project{
				Description: projectInfo.Description,
				Name:        projectInfo.Name,
				OwnerID:     auth.User.ID,
				PartnerID:   auth.User.PartnerID,
			},
		)
		if err != nil {
			return Error.Wrap(err)
		}

		_, err = tx.ProjectMembers().Insert(ctx, auth.User.ID, p.ID)
		if err != nil {
			return Error.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// ToDo: check if this is actually the right place.
	err = s.accounts.Coupons().AddPromotionalCoupon(ctx, auth.User.ID)
	if err != nil {
		s.log.Debug(fmt.Sprintf("could not add promotional coupon for user %s", auth.User.ID.String()), zap.Error(Error.Wrap(err)))
	}

	return p, nil
}

// DeleteProject is a method for deleting project by id.
func (s *Service) DeleteProject(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "delete project", zap.String("projectID", projectID.String()))
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = s.isProjectOwner(ctx, auth.User.ID, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.checkProjectCanBeDeleted(ctx, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.store.Projects().Delete(ctx, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// UpdateProject is a method for updating project name and description by id.
func (s *Service) UpdateProject(ctx context.Context, projectID uuid.UUID, name string, description string) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "update project name and description", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = ValidateNameAndDescription(name, description)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	project := isMember.project
	project.Name = name
	project.Description = description

	err = s.store.Projects().Update(ctx, project)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return project, nil
}

// AddProjectMembers adds users by email to given project.
func (s *Service) AddProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (users []*User, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "add project members", zap.String("projectID", projectID.String()), zap.Strings("emails", emails))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if _, err = s.isProjectMember(ctx, auth.User.ID, projectID); err != nil {
		return nil, Error.Wrap(err)
	}

	var userErr errs.Group

	// collect user querying errors
	for _, email := range emails {
		user, err := s.store.Users().GetByEmail(ctx, email)
		if err != nil {
			userErr.Add(err)
			continue
		}

		users = append(users, user)
	}

	if err = userErr.Err(); err != nil {
		return nil, ErrValidation.New(teamMemberDoesNotExistErrMsg)
	}

	// add project members in transaction scope
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		for _, user := range users {
			if _, err := tx.ProjectMembers().Insert(ctx, user.ID, projectID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return users, nil
}

// DeleteProjectMembers removes users by email from given project.
func (s *Service) DeleteProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := s.getAuthAndAuditLog(ctx, "delete project members", zap.String("projectID", projectID.String()), zap.Strings("emails", emails))
	if err != nil {
		return Error.Wrap(err)
	}

	if _, err = s.isProjectMember(ctx, auth.User.ID, projectID); err != nil {
		return Error.Wrap(err)
	}

	var userIDs []uuid.UUID
	var userErr errs.Group

	// collect user querying errors
	for _, email := range emails {
		user, err := s.store.Users().GetByEmail(ctx, email)

		if err != nil {
			userErr.Add(err)
			continue
		}

		isOwner, err := s.isProjectOwner(ctx, user.ID, projectID)
		if isOwner {
			return ErrValidation.New(projectOwnerDeletionForbiddenErrMsg, user.Email)
		}
		if err != nil && !ErrUnauthorized.Has(err) {
			return Error.Wrap(err)
		}

		userIDs = append(userIDs, user.ID)
	}

	if err = userErr.Err(); err != nil {
		return ErrValidation.New(teamMemberDoesNotExistErrMsg)
	}

	// delete project members in transaction scope
	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		for _, uID := range userIDs {
			err = tx.ProjectMembers().Delete(ctx, uID, projectID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return Error.Wrap(err)
}

// GetProjectMembers returns ProjectMembers for given Project.
func (s *Service) GetProjectMembers(ctx context.Context, projectID uuid.UUID, cursor ProjectMembersCursor) (pmp *ProjectMembersPage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get project members", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}

	pmp, err = s.store.ProjectMembers().GetPagedByProjectID(ctx, projectID, cursor)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// CreateAPIKey creates new api key.
func (s *Service) CreateAPIKey(ctx context.Context, projectID uuid.UUID, name string) (_ *APIKeyInfo, _ *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "create api key", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	_, err = s.store.APIKeys().GetByNameAndProjectID(ctx, name, projectID)
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
		ProjectID: projectID,
		Secret:    secret,
		PartnerID: auth.User.PartnerID,
	}

	info, err := s.store.APIKeys().Create(ctx, key.Head(), apikey)
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	return info, key, nil
}

// GetAPIKeyInfo retrieves api key by id.
func (s *Service) GetAPIKeyInfo(ctx context.Context, id uuid.UUID) (_ *APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get api key info", zap.String("apiKeyID", id.String()))
	if err != nil {
		return nil, err
	}

	key, err := s.store.APIKeys().Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, key.ProjectID)
	if err != nil {
		return nil, Error.Wrap(err)
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

	auth, err := s.getAuthAndAuditLog(ctx, "delete api keys", zap.Strings("apiKeyIDs", idStrings))
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

		_, err = s.isProjectMember(ctx, auth.User.ID, key.ProjectID)
		if err != nil {
			keysErr.Add(ErrUnauthorized.Wrap(err))
			continue
		}
	}

	if err = keysErr.Err(); err != nil {
		return Error.Wrap(err)
	}

	err = s.store.WithTx(ctx, func(ctx context.Context, tx DBTx) error {
		for _, keyToDeleteID := range ids {
			err = tx.APIKeys().Delete(ctx, keyToDeleteID)
			if err != nil {
				return err
			}
		}

		return nil
	})
	return Error.Wrap(err)
}

// GetAPIKeys returns paged api key list for given Project.
func (s *Service) GetAPIKeys(ctx context.Context, projectID uuid.UUID, cursor APIKeyCursor) (page *APIKeyPage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get api keys", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if cursor.Limit > maxLimit {
		cursor.Limit = maxLimit
	}

	page, err = s.store.APIKeys().GetPagedByProjectID(ctx, projectID, cursor)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// GetProjectUsage retrieves project usage for a given period.
func (s *Service) GetProjectUsage(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ *accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get project usage", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	projectUsage, err := s.projectAccounting.GetProjectTotal(ctx, projectID, since, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return projectUsage, nil
}

// GetBucketTotals retrieves paged bucket total usages since project creation.
func (s *Service) GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor accounting.BucketUsageCursor, before time.Time) (_ *accounting.BucketUsagePage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get bucket totals", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	isMember, err := s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	usage, err := s.projectAccounting.GetBucketTotals(ctx, projectID, cursor, isMember.project.CreatedAt, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return usage, nil
}

// GetAllBucketNames retrieves all bucket names of a specific project.
func (s *Service) GetAllBucketNames(ctx context.Context, projectID uuid.UUID) (_ []string, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get all bucket names", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	listOptions := storj.BucketListOptions{
		Direction: storj.Forward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	bucketsList, err := s.buckets.ListBuckets(ctx, projectID, listOptions, allowedBuckets)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	var list []string
	for _, bucket := range bucketsList.Items {
		list = append(list, bucket.Name)
	}

	return list, nil
}

// GetBucketUsageRollups retrieves summed usage rollups for every bucket of particular project for a given period.
func (s *Service) GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ []accounting.BucketUsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := s.getAuthAndAuditLog(ctx, "get bucket usage rollups", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	result, err := s.projectAccounting.GetBucketUsageRollups(ctx, projectID, since, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// GetProjectUsageLimits returns project limits and current usage.
//
// Among others,it can return one of the following errors returned by
// storj.io/storj/satellite/accounting.Service, wrapped Error.
func (s *Service) GetProjectUsageLimits(ctx context.Context, projectID uuid.UUID) (_ *ProjectUsageLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = s.getAuthAndAuditLog(ctx, "get project usage limits", zap.String("projectID", projectID.String()))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	storageLimit, err := s.projectUsage.GetProjectStorageLimit(ctx, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	bandwidthLimit, err := s.projectUsage.GetProjectBandwidthLimit(ctx, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	storageUsed, err := s.projectUsage.GetProjectStorageTotals(ctx, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	bandwidthUsed, err := s.projectUsage.GetProjectBandwidthTotals(ctx, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &ProjectUsageLimits{
		StorageLimit:   storageLimit.Int64(),
		BandwidthLimit: bandwidthLimit.Int64(),
		StorageUsed:    storageUsed,
		BandwidthUsed:  bandwidthUsed,
	}, nil
}

// Authorize validates token from context and returns authorized Authorization.
func (s *Service) Authorize(ctx context.Context) (a Authorization, err error) {
	defer mon.Task()(&ctx)(&err)
	tokenS, ok := consoleauth.GetAPIKey(ctx)
	if !ok {
		return Authorization{}, ErrUnauthorized.New("no api key was provided")
	}

	token, err := consoleauth.FromBase64URLString(string(tokenS))
	if err != nil {
		return Authorization{}, ErrUnauthorized.Wrap(err)
	}

	claims, err := s.authenticate(ctx, token)
	if err != nil {
		return Authorization{}, ErrUnauthorized.Wrap(err)
	}

	user, err := s.authorize(ctx, claims)
	if err != nil {
		return Authorization{}, ErrUnauthorized.Wrap(err)
	}

	return Authorization{
		User:   *user,
		Claims: *claims,
	}, nil
}

// checkProjectCanBeDeleted ensures that all data, api-keys and buckets are deleted and usage has been accounted.
// no error means the project status is clean.
func (s *Service) checkProjectCanBeDeleted(ctx context.Context, project uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	buckets, err := s.buckets.CountBuckets(ctx, project)
	if err != nil {
		return err
	}
	if buckets > 0 {
		return ErrUsage.New("some buckets still exist")
	}

	keys, err := s.store.APIKeys().GetPagedByProjectID(ctx, project, APIKeyCursor{Limit: 1, Page: 1})
	if err != nil {
		return err
	}
	if keys.TotalCount > 0 {
		return ErrUsage.New("some api-keys still exist")
	}

	outstanding, err := s.Payments().checkProjectInvoicingStatus(ctx, project)
	if outstanding {
		return ErrUsage.New("there is outstanding usage that is not charged yet")
	}
	return ErrUsage.Wrap(err)
}

// checkProjectLimit is used to check if user is able to create a new project.
func (s *Service) checkProjectLimit(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	limit, err := s.store.Users().GetProjectLimit(ctx, userID)
	if err != nil {
		return Error.Wrap(err)
	}
	if limit == 0 {
		limit = s.config.DefaultProjectLimit
	}

	projects, err := s.GetUsersProjects(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	if len(projects) >= limit {
		return ErrProjLimit.New(projLimitErrMsg)
	}

	return nil
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

// createToken creates string representation.
func (s *Service) createToken(ctx context.Context, claims *consoleauth.Claims) (_ string, err error) {
	defer mon.Task()(&ctx)(&err)

	json, err := claims.JSON()
	if err != nil {
		return "", Error.Wrap(err)
	}

	token := consoleauth.Token{Payload: json}
	err = signToken(&token, s.Signer)
	if err != nil {
		return "", Error.Wrap(err)
	}

	return token.String(), nil
}

// authenticate validates token signature and returns authenticated *satelliteauth.Authorization.
func (s *Service) authenticate(ctx context.Context, token consoleauth.Token) (_ *consoleauth.Claims, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := token.Signature

	err = signToken(&token, s.Signer)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if subtle.ConstantTimeCompare(signature, token.Signature) != 1 {
		return nil, Error.New("incorrect signature")
	}

	claims, err := consoleauth.FromJSON(token.Payload)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return claims, nil
}

// authorize checks claims and returns authorized User.
func (s *Service) authorize(ctx context.Context, claims *consoleauth.Claims) (_ *User, err error) {
	defer mon.Task()(&ctx)(&err)
	if !claims.Expiration.IsZero() && claims.Expiration.Before(time.Now()) {
		return nil, ErrTokenExpiration.New("")
	}

	user, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return nil, ErrValidation.New("authorization failed. no user with id: %s", claims.ID.String())
	}

	return user, nil
}

// isProjectMember is return type of isProjectMember service method.
type isProjectMember struct {
	project    *Project
	membership *ProjectMember
}

// isProjectOwner checks if the user is an owner of a project.
func (s *Service) isProjectOwner(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (isOwner bool, err error) {
	defer mon.Task()(&ctx)(&err)
	project, err := s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return false, err
	}

	if project.OwnerID != userID {
		return false, ErrUnauthorized.New(unauthorizedErrMsg)
	}

	return true, nil
}

// isProjectMember checks if the user is a member of given project.
func (s *Service) isProjectMember(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (_ isProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	project, err := s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return isProjectMember{}, Error.Wrap(err)
	}

	memberships, err := s.store.ProjectMembers().GetByMemberID(ctx, userID)
	if err != nil {
		return isProjectMember{}, Error.Wrap(err)
	}

	membership, ok := findMembershipByProjectID(memberships, projectID)
	if ok {
		return isProjectMember{
			project:    project,
			membership: &membership,
		}, nil
	}

	return isProjectMember{}, ErrNoMembership.New(unauthorizedErrMsg)
}

func findMembershipByProjectID(memberships []ProjectMember, projectID uuid.UUID) (ProjectMember, bool) {
	for _, membership := range memberships {
		if membership.ProjectID == projectID {
			return membership, true
		}
	}
	return ProjectMember{}, false
}

// PaywallEnabled returns a true if a credit card or account
// balance is required to create projects.
func (s *Service) PaywallEnabled(userID uuid.UUID) bool {
	return s.accounts.PaywallEnabled(userID)
}
