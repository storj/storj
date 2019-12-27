// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/subtle"
	"sort"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/macaroon"
	"storj.io/storj/pkg/auth"
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

// Error messages
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

	// TODO: remove after vanguard release
	usedRegTokenVanguardErrMsg = "This registration token has already been used"
	projLimitVanguardErrMsg    = "Sorry, during the Vanguard release you have a limited number of projects"
)

// Error describes internal console error.
var Error = errs.Class("service error")

// ErrNoMembership is error type of not belonging to a specific project.
var ErrNoMembership = errs.Class("no membership error")

// ErrTokenExpiration is error type of token reached expiration time.
var ErrTokenExpiration = errs.Class("token expiration error")

// ErrProjLimit is error type of project limit.
var ErrProjLimit = errs.Class("project limit error")

// Service is handling accounts related logic
//
// architecture: Service
type Service struct {
	Signer

	log               *zap.Logger
	store             DB
	projectAccounting accounting.ProjectAccounting
	projectUsage      *accounting.Service
	rewards           rewards.DB
	partners          *rewards.PartnersService
	accounts          payments.Accounts

	passwordCost int
}

// PaymentsService separates all payment related functionality
type PaymentsService struct {
	service *Service
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, signer Signer, store DB, projectAccounting accounting.ProjectAccounting, projectUsage *accounting.Service, rewards rewards.DB, partners *rewards.PartnersService, accounts payments.Accounts, passwordCost int) (*Service, error) {
	if signer == nil {
		return nil, errs.New("signer can't be nil")
	}
	if store == nil {
		return nil, errs.New("store can't be nil")
	}
	if log == nil {
		return nil, errs.New("log can't be nil")
	}
	if passwordCost == 0 {
		passwordCost = bcrypt.DefaultCost
	}

	return &Service{
		log:               log,
		Signer:            signer,
		store:             store,
		projectAccounting: projectAccounting,
		projectUsage:      projectUsage,
		rewards:           rewards,
		partners:          partners,
		accounts:          accounts,
		passwordCost:      passwordCost,
	}, nil
}

// Payments separates all payment related functionality
func (s *Service) Payments() PaymentsService {
	return PaymentsService{service: s}
}

// SetupAccount creates payment account for authorized user.
func (payments PaymentsService) SetupAccount(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return payments.service.accounts.Setup(ctx, auth.User.ID, auth.User.Email)
}

// AccountBalance return account balance.
func (payments PaymentsService) AccountBalance(ctx context.Context) (balance int64, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return 0, err
	}

	return payments.service.accounts.Balance(ctx, auth.User.ID)
}

// AddCreditCard is used to save new credit card and attach it to payment account.
func (payments PaymentsService) AddCreditCard(ctx context.Context, creditCardToken string) (err error) {
	defer mon.Task()(&ctx, creditCardToken)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return payments.service.accounts.CreditCards().Add(ctx, auth.User.ID, creditCardToken)
}

// MakeCreditCardDefault makes a credit card default payment method.
func (payments PaymentsService) MakeCreditCardDefault(ctx context.Context, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return payments.service.accounts.CreditCards().MakeDefault(ctx, auth.User.ID, cardID)
}

// ProjectsCharges returns how much money current user will be charged for each project which he owns.
func (payments PaymentsService) ProjectsCharges(ctx context.Context) (_ []payments.ProjectCharge, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return payments.service.accounts.ProjectCharges(ctx, auth.User.ID)
}

// ListCreditCards returns a list of credit cards for a given payment account.
func (payments PaymentsService) ListCreditCards(ctx context.Context) (_ []payments.CreditCard, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return payments.service.accounts.CreditCards().List(ctx, auth.User.ID)
}

// RemoveCreditCard is used to detach a credit card from payment account.
func (payments PaymentsService) RemoveCreditCard(ctx context.Context, cardID string) (err error) {
	defer mon.Task()(&ctx, cardID)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return payments.service.accounts.CreditCards().Remove(ctx, auth.User.ID, cardID)
}

// BillingHistory returns a list of invoices, transactions and all others billing history items for payment account.
func (payments PaymentsService) BillingHistory(ctx context.Context) (billingHistory []*BillingHistoryItem, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	invoices, err := payments.service.accounts.Invoices().List(ctx, auth.User.ID)
	if err != nil {
		return nil, err
	}

	// TODO: add transactions, etc in future
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

	txsInfos, err := payments.service.accounts.StorjTokens().ListTransactionInfos(ctx, auth.User.ID)
	if err != nil {
		return nil, err
	}

	for _, info := range txsInfos {
		billingHistory = append(billingHistory,
			&BillingHistoryItem{
				ID:          info.ID.String(),
				Description: "STORJ Token Deposit",
				Amount:      info.AmountCents,
				Received:    info.ReceivedCents,
				Status:      info.Status.String(),
				Link:        info.Link,
				Start:       info.CreatedAt,
				End:         info.ExpiresAt,
				Type:        Transaction,
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
func (payments PaymentsService) TokenDeposit(ctx context.Context, amount int64) (_ *payments.Transaction, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	tx, err := payments.service.accounts.StorjTokens().Deposit(ctx, auth.User.ID, amount)
	return tx, errs.Wrap(err)
}

// CreateUser gets password hash value and creates new inactive User
func (s *Service) CreateUser(ctx context.Context, user CreateUser, tokenSecret RegistrationSecret, refUserID string) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)
	if err := user.IsValid(); err != nil {
		return nil, err
	}

	offerType := rewards.FreeCredit
	if user.PartnerID != "" {
		offerType = rewards.Partner
	} else if refUserID != "" {
		offerType = rewards.Referral
	}

	//TODO: Create a current offer cache to replace database call
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

	// TODO: remove after vanguard release
	// when user uses an open source partner referral link, there won't be a registration token in the link.
	// therefore, we need to create one so we can still control the project limit on the account level
	var registrationToken *RegistrationToken
	if user.PartnerID != "" {
		// set the project limit to be 1 for open source partner invitees
		registrationToken, err = s.store.RegistrationTokens().Create(ctx, 1)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	} else {
		registrationToken, err = s.store.RegistrationTokens().GetBySecret(ctx, tokenSecret)
		if err != nil {
			return nil, ErrUnauthorized.Wrap(err)
		}
		// if a registration token is already associated with an user ID, that means the token is already used
		// we should terminate the account creation process and return an error
		if registrationToken.OwnerID != nil {
			return nil, errs.New(usedRegTokenVanguardErrMsg)
		}
	}

	u, err = s.store.Users().GetByEmail(ctx, user.Email)
	if err == nil {
		return nil, errs.New(emailUsedErrMsg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), s.passwordCost)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	// store data
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, err
	}

	err = withTx(tx, func(tx DBTx) error {
		userID, err := uuid.New()
		if err != nil {
			return Error.Wrap(err)
		}

		newUser := &User{
			ID:           *userID,
			Email:        user.Email,
			FullName:     user.FullName,
			ShortName:    user.ShortName,
			PasswordHash: hash,
		}
		if user.PartnerID != "" {
			partnerID, err := uuid.Parse(user.PartnerID)
			if err != nil {
				return Error.Wrap(err)
			}
			newUser.PartnerID = *partnerID
		}

		u, err = tx.Users().Insert(ctx,
			newUser,
		)
		if err != nil {
			return Error.Wrap(err)
		}

		err = tx.RegistrationTokens().UpdateOwner(ctx, registrationToken.Secret, u.ID)
		if err != nil {
			return Error.Wrap(err)
		}

		if currentReward != nil {
			_ = currentReward
			// NB: Uncomment this block when UserCredits().Create is cockroach compatible
			// var refID *uuid.UUID
			// if refUserID != "" {
			// 	refID, err = uuid.Parse(refUserID)
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
		return nil, err
	}

	return u, nil
}

// GenerateActivationToken - is a method for generating activation token
func (s *Service) GenerateActivationToken(ctx context.Context, id uuid.UUID, email string) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	//TODO: activation token should differ from auth token
	claims := &consoleauth.Claims{
		ID:         id,
		Email:      email,
		Expiration: time.Now().Add(time.Hour * 24),
	}

	return s.createToken(ctx, claims)
}

// GeneratePasswordRecoveryToken - is a method for generating password recovery token
func (s *Service) GeneratePasswordRecoveryToken(ctx context.Context, id uuid.UUID) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	resetPasswordToken, err := s.store.ResetPasswordTokens().GetByOwnerID(ctx, id)
	if err == nil {
		err := s.store.ResetPasswordTokens().Delete(ctx, resetPasswordToken.Secret)
		if err != nil {
			return "", err
		}
	}

	resetPasswordToken, err = s.store.ResetPasswordTokens().Create(ctx, id)
	if err != nil {
		return "", err
	}

	return resetPasswordToken.Secret.String(), nil
}

// ActivateAccount - is a method for activating user account after registration
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
		return errs.New(emailUsedErrMsg)
	}

	user, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	now := time.Now()

	if user.Status == Active {
		return errs.New("account is already active")
	}

	if now.After(user.CreatedAt.Add(tokenExpirationTime)) {
		return ErrTokenExpiration.Wrap(err)
	}

	user.Status = Active
	err = s.store.Users().Update(ctx, user)
	if err != nil {
		return Error.Wrap(err)
	}

	err = s.store.UserCredits().UpdateEarnedCredits(ctx, user.ID)
	if err != nil && !NoCreditForUpdateErr.Has(err) {
		return Error.Wrap(err)
	}

	return nil
}

// ResetPassword - is a method for reseting user password
func (s *Service) ResetPassword(ctx context.Context, resetPasswordToken, password string) (err error) {
	defer mon.Task()(&ctx)(&err)

	secret, err := ResetPasswordSecretFromBase64(resetPasswordToken)
	if err != nil {
		return err
	}
	token, err := s.store.ResetPasswordTokens().GetBySecret(ctx, secret)
	if err != nil {
		return err
	}

	user, err := s.store.Users().Get(ctx, *token.OwnerID)
	if err != nil {
		return err
	}

	if err := ValidatePassword(password); err != nil {
		return err
	}

	if time.Since(token.CreatedAt) > tokenExpirationTime {
		return errs.New(passwordRecoveryTokenIsExpiredErrMsg)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.passwordCost)
	if err != nil {
		return err
	}

	user.PasswordHash = hash

	err = s.store.Users().Update(ctx, user)
	if err != nil {
		return err
	}

	if err = s.store.ResetPasswordTokens().Delete(ctx, token.Secret); err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// RevokeResetPasswordToken - is a method to revoke reset password token
func (s *Service) RevokeResetPasswordToken(ctx context.Context, resetPasswordToken string) (err error) {
	defer mon.Task()(&ctx)(&err)

	secret, err := ResetPasswordSecretFromBase64(resetPasswordToken)
	if err != nil {
		return
	}

	return s.store.ResetPasswordTokens().Delete(ctx, secret)
}

// Token authenticates User by credentials and returns auth token
func (s *Service) Token(ctx context.Context, email, password string) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.store.Users().GetByEmail(ctx, email)
	if err != nil {
		return "", ErrUnauthorized.New(credentialsErrMsg)
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return "", ErrUnauthorized.New(credentialsErrMsg)
		}

		return "", Error.Wrap(err)
	}

	claims := consoleauth.Claims{
		ID:         user.ID,
		Expiration: time.Now().Add(tokenExpirationTime),
	}

	token, err = s.createToken(ctx, &claims)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GetUser returns User by id
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.store.Users().Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return user, nil
}

// GetUserByEmail returns User by email
func (s *Service) GetUserByEmail(ctx context.Context, email string) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)

	result, err := s.store.Users().GetByEmail(ctx, email)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// UpdateAccount updates User
func (s *Service) UpdateAccount(ctx context.Context, fullName string, shortName string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
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

// ChangePassword updates password for a given user
func (s *Service) ChangePassword(ctx context.Context, pass, newPass string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	err = bcrypt.CompareHashAndPassword(auth.User.PasswordHash, []byte(pass))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return ErrUnauthorized.Wrap(err)
		}

		return Error.Wrap(err)
	}

	if err := ValidatePassword(newPass); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), s.passwordCost)
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

// DeleteAccount deletes User
func (s *Service) DeleteAccount(ctx context.Context, password string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	err = bcrypt.CompareHashAndPassword(auth.User.PasswordHash, []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return ErrUnauthorized.Wrap(err)
		}

		return Error.Wrap(err)
	}

	err = s.store.Users().Delete(ctx, auth.User.ID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// GetProject is a method for querying project by id
func (s *Service) GetProject(ctx context.Context, projectID uuid.UUID) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	p, err = s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// GetUsersProjects is a method for querying all projects
func (s *Service) GetUsersProjects(ctx context.Context) (ps []Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	ps, err = s.store.Projects().GetByUserID(ctx, auth.User.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return
}

// GetCurrentRewardByType is a method for querying current active reward offer based on its type
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

// GetUserCreditUsage is a method for querying users' credit information up until now
func (s *Service) GetUserCreditUsage(ctx context.Context) (usage *UserCreditUsage, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	usage, err = s.store.UserCredits().GetCreditUsage(ctx, auth.User.ID, time.Now().UTC())
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return usage, nil
}

// CreateProject is a method for creating new project
func (s *Service) CreateProject(ctx context.Context, projectInfo ProjectInfo) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	// TODO: remove after vanguard release
	err = s.checkProjectLimit(ctx, auth.User.ID)
	if err != nil {
		return nil, ErrProjLimit.Wrap(err)
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	err = withTx(tx, func(tx DBTx) error {
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
		return nil, err
	}

	return p, nil
}

// DeleteProject is a method for deleting project by id
func (s *Service) DeleteProject(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	if _, err = s.isProjectMember(ctx, auth.User.ID, projectID); err != nil {
		return ErrUnauthorized.Wrap(err)
	}

	err = s.store.Projects().Delete(ctx, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// UpdateProject is a method for updating project description by id
func (s *Service) UpdateProject(ctx context.Context, projectID uuid.UUID, description string) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	isMember, err := s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		if ErrUnauthorized.Has(err) {
			return nil, ErrUnauthorized.Wrap(err)
		}

		return nil, Error.Wrap(err)
	}

	project := isMember.project
	project.Description = description

	err = s.store.Projects().Update(ctx, project)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return project, nil
}

// AddProjectMembers adds users by email to given project
func (s *Service) AddProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (users []*User, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	if _, err = s.isProjectMember(ctx, auth.User.ID, projectID); err != nil {
		if ErrUnauthorized.Has(err) {
			return nil, ErrUnauthorized.Wrap(err)
		}

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
		return nil, errs.New(teamMemberDoesNotExistErrMsg)
	}

	// add project members in transaction scope
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()

	for _, user := range users {
		_, err = tx.ProjectMembers().Insert(ctx, user.ID, projectID)

		if err != nil {
			return nil, Error.Wrap(err)
		}
	}

	return users, nil
}

// DeleteProjectMembers removes users by email from given project
func (s *Service) DeleteProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return err
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

		err = s.isProjectOwner(ctx, user.ID, projectID)
		if err == nil {
			return errs.New(projectOwnerDeletionForbiddenErrMsg, user.Email)
		}

		if Error.Has(err) {
			return err
		}

		userIDs = append(userIDs, user.ID)
	}

	if err = userErr.Err(); err != nil {
		return errs.New(teamMemberDoesNotExistErrMsg)
	}

	// delete project members in transaction scope
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()

	for _, uID := range userIDs {
		err = tx.ProjectMembers().Delete(ctx, uID, projectID)

		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// GetProjectMembers returns ProjectMembers for given Project
func (s *Service) GetProjectMembers(ctx context.Context, projectID uuid.UUID, cursor ProjectMembersCursor) (pmp *ProjectMembersPage, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		if ErrUnauthorized.Has(err) {
			return nil, ErrUnauthorized.Wrap(err)
		}

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

// CreateAPIKey creates new api key
func (s *Service) CreateAPIKey(ctx context.Context, projectID uuid.UUID, name string) (_ *APIKeyInfo, _ *macaroon.APIKey, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, nil, ErrUnauthorized.Wrap(err)
	}

	_, err = s.store.APIKeys().GetByNameAndProjectID(ctx, name, projectID)
	if err == nil {
		return nil, nil, errs.New(apiKeyWithNameExistsErrMsg)
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

// GetAPIKeyInfo retrieves api key by id
func (s *Service) GetAPIKeyInfo(ctx context.Context, id uuid.UUID) (_ *APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	key, err := s.store.APIKeys().Get(ctx, id)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, key.ProjectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	return key, nil
}

// DeleteAPIKeys deletes api key by id
func (s *Service) DeleteAPIKeys(ctx context.Context, ids []uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
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

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()

	for _, keyToDeleteID := range ids {
		err = tx.APIKeys().Delete(ctx, keyToDeleteID)
		if err != nil {
			return Error.Wrap(err)
		}
	}

	return nil
}

// GetAPIKeys returns paged api key list for given Project
func (s *Service) GetAPIKeys(ctx context.Context, projectID uuid.UUID, cursor APIKeyCursor) (page *APIKeyPage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
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

// GetProjectUsage retrieves project usage for a given period
func (s *Service) GetProjectUsage(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ *accounting.ProjectUsage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	projectUsage, err := s.projectAccounting.GetProjectTotal(ctx, projectID, since, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return projectUsage, nil
}

// GetBucketTotals retrieves paged bucket total usages since project creation
func (s *Service) GetBucketTotals(ctx context.Context, projectID uuid.UUID, cursor accounting.BucketUsageCursor, before time.Time) (_ *accounting.BucketUsagePage, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	isMember, err := s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	usage, err := s.projectAccounting.GetBucketTotals(ctx, projectID, cursor, isMember.project.CreatedAt, before)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return usage, nil
}

// GetBucketUsageRollups retrieves summed usage rollups for every bucket of particular project for a given period
func (s *Service) GetBucketUsageRollups(ctx context.Context, projectID uuid.UUID, since, before time.Time) (_ []accounting.BucketUsageRollup, err error) {
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	result, err := s.projectAccounting.GetBucketUsageRollups(ctx, projectID, since, before)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetProjectUsageLimits returns project limits and current usage.
func (s *Service) GetProjectUsageLimits(ctx context.Context, projectID uuid.UUID) (_ *ProjectUsageLimits, err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	storageLimit, err := s.projectUsage.GetProjectStorageLimit(ctx, projectID)
	if err != nil {
		return nil, err
	}
	bandwidthLimit, err := s.projectUsage.GetProjectBandwidthLimit(ctx, projectID)
	if err != nil {
		return nil, err
	}

	storageUsed, err := s.projectUsage.GetProjectStorageTotals(ctx, projectID)
	if err != nil {
		return nil, err
	}
	bandwidthUsed, err := s.projectUsage.GetProjectBandwidthTotals(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &ProjectUsageLimits{
		StorageLimit:   storageLimit.Int64(),
		BandwidthLimit: bandwidthLimit.Int64(),
		StorageUsed:    storageUsed,
		BandwidthUsed:  bandwidthUsed,
	}, nil
}

// Authorize validates token from context and returns authorized Authorization
func (s *Service) Authorize(ctx context.Context) (a Authorization, err error) {
	defer mon.Task()(&ctx)(&err)
	tokenS, ok := auth.GetAPIKey(ctx)
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
		if ErrUnauthorized.Has(err) {
			return Authorization{}, ErrUnauthorized.Wrap(err)
		}

		return Authorization{}, ErrTokenExpiration.Wrap(err)
	}

	return Authorization{
		User:   *user,
		Claims: *claims,
	}, nil
}

// checkProjectLimit is used to check if user is able to create a new project
func (s *Service) checkProjectLimit(ctx context.Context, userID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	registrationToken, err := s.store.RegistrationTokens().GetByOwnerID(ctx, userID)
	if err != nil {
		return err
	}

	projects, err := s.GetUsersProjects(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	if len(projects) >= registrationToken.ProjectLimit {
		return ErrProjLimit.Wrap(errs.New(projLimitVanguardErrMsg))
	}

	return nil
}

// CreateRegToken creates new registration token. Needed for testing
func (s *Service) CreateRegToken(ctx context.Context, projLimit int) (_ *RegistrationToken, err error) {
	defer mon.Task()(&ctx)(&err)
	result, err := s.store.RegistrationTokens().Create(ctx, projLimit)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return result, nil
}

// createToken creates string representation
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

// authenticate validates token signature and returns authenticated *satelliteauth.Authorization
func (s *Service) authenticate(ctx context.Context, token consoleauth.Token) (_ *consoleauth.Claims, err error) {
	defer mon.Task()(&ctx)(&err)
	signature := token.Signature

	err = signToken(&token, s.Signer)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if subtle.ConstantTimeCompare(signature, token.Signature) != 1 {
		return nil, errs.New("incorrect signature")
	}

	claims, err := consoleauth.FromJSON(token.Payload)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return claims, nil
}

// authorize checks claims and returns authorized User
func (s *Service) authorize(ctx context.Context, claims *consoleauth.Claims) (_ *User, err error) {
	defer mon.Task()(&ctx)(&err)
	if !claims.Expiration.IsZero() && claims.Expiration.Before(time.Now()) {
		return nil, errs.New("token is outdated")
	}

	user, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return nil, errs.New("authorization failed. no user with id: %s", claims.ID.String())
	}

	return user, nil
}

// isProjectMember is return type of isProjectMember service method
type isProjectMember struct {
	project    *Project
	membership *ProjectMember
}

// isProjectOwner checks if the user is an owner of a project
func (s *Service) isProjectOwner(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	project, err := s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return Error.Wrap(err)
	}

	if project.OwnerID != userID {
		return errs.New(unauthorizedErrMsg)
	}

	return nil
}

// isProjectMember checks if the user is a member of given project
func (s *Service) isProjectMember(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (result isProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	project, err := s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return result, Error.Wrap(err)
	}

	memberships, err := s.store.ProjectMembers().GetByMemberID(ctx, userID)
	if err != nil {
		return result, Error.Wrap(err)
	}

	for _, membership := range memberships {
		if membership.ProjectID == projectID {
			result.membership = &membership // nolint: scopelint
			result.project = project
			return
		}
	}

	return isProjectMember{}, ErrNoMembership.New(unauthorizedErrMsg)
}

// withTx is a helper function for executing db operations
// in transaction scope
func withTx(tx DBTx, cb func(tx DBTx) error) (err error) {
	defer func() {
		if err != nil {
			err = errs.Combine(err, tx.Rollback())
			return
		}

		err = tx.Commit()
	}()

	return cb(tx)
}
