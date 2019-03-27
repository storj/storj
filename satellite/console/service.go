// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console/consoleauth"
)

var (
	mon = monkit.Package()
)

const (
	// maxLimit specifies the limit for all paged queries
	maxLimit            = 50
	tokenExpirationTime = 24 * time.Hour

	// DefaultPasswordCost is the hashing complexity
	DefaultPasswordCost = bcrypt.DefaultCost
	// TestPasswordCost is the hashing complexity to use for testing
	TestPasswordCost = bcrypt.MinCost
)

// Service is handling accounts related logic
type Service struct {
	Signer

	store DB
	log   *zap.Logger

	passwordCost int
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, signer Signer, store DB, passwordCost int) (*Service, error) {
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
		Signer:       signer,
		store:        store,
		log:          log,
		passwordCost: passwordCost,
	}, nil
}

// CreateUser gets password hash value and creates new inactive User
func (s *Service) CreateUser(ctx context.Context, user CreateUser, tokenSecret RegistrationSecret) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)
	if err := user.IsValid(); err != nil {
		return nil, err
	}

	// TODO: remove after vanguard release
	registrationToken, err := s.store.RegistrationTokens().GetBySecret(ctx, tokenSecret)
	if err != nil {
		return nil, err
	}
	if registrationToken.OwnerID != nil {
		return nil, errs.New("token is already used")
	}

	// TODO: store original email input in the db,
	// add normalization
	email := normalizeEmail(user.Email)

	u, err = s.store.Users().GetByEmail(ctx, email)
	if err == nil {
		return nil, errs.New(fmt.Sprintf("%s is already in use", email))
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), s.passwordCost)
	if err != nil {
		return nil, err
	}

	u, err = s.store.Users().Insert(ctx, &User{
		Email:        user.Email,
		FullName:     user.FullName,
		ShortName:    user.ShortName,
		PasswordHash: hash,
	})

	err = s.store.RegistrationTokens().UpdateOwner(ctx, registrationToken.Secret, u.ID)
	if err != nil {
		return nil, err
	}

	return u, err
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

	return s.createToken(claims)
}

// ActivateAccount - is a method for activating user account after registration
func (s *Service) ActivateAccount(ctx context.Context, activationToken string) (err error) {
	defer mon.Task()(&ctx)(&err)

	token, err := consoleauth.FromBase64URLString(activationToken)
	if err != nil {
		return
	}

	claims, err := s.authenticate(token)
	if err != nil {
		return
	}

	user, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return
	}

	now := time.Now()

	if user.Status != Inactive {
		return errs.New("account is already active")
	}

	if now.After(user.CreatedAt.Add(tokenExpirationTime)) {
		return errs.New("activation token has expired")
	}

	user.Status = Active

	return s.store.Users().Update(ctx, user)
}

// Token authenticates User by credentials and returns auth token
func (s *Service) Token(ctx context.Context, email, password string) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	email = normalizeEmail(email)

	user, err := s.store.Users().GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password))
	if err != nil {
		return "", ErrUnauthorized.New("password is incorrect: %s", err.Error())
	}

	claims := consoleauth.Claims{
		ID:         user.ID,
		Expiration: time.Now().Add(tokenExpirationTime),
	}

	token, err = s.createToken(&claims)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GetUser returns User by id
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (u *User, err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Users().Get(ctx, id)
}

// UpdateAccount updates User
func (s *Service) UpdateAccount(ctx context.Context, info UserInfo) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	if err = info.IsValid(); err != nil {
		return err
	}

	//TODO: store original email input in the db,
	// add normalization
	email := normalizeEmail(info.Email)

	return s.store.Users().Update(ctx, &User{
		ID:           auth.User.ID,
		FullName:     info.FullName,
		ShortName:    info.ShortName,
		Email:        email,
		PasswordHash: nil,
	})
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
		return ErrUnauthorized.New("origin password is incorrect: %s", err.Error())
	}

	if err := validatePassword(newPass); err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), s.passwordCost)
	if err != nil {
		return err
	}

	auth.User.PasswordHash = hash
	return s.store.Users().Update(ctx, &auth.User)
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
		return ErrUnauthorized.New("origin password is incorrect")
	}

	return s.store.Users().Delete(ctx, auth.User.ID)
}

// GetProject is a method for querying project by id
func (s *Service) GetProject(ctx context.Context, projectID uuid.UUID) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Projects().Get(ctx, projectID)
}

// GetUsersProjects is a method for querying all projects
func (s *Service) GetUsersProjects(ctx context.Context) (ps []Project, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Projects().GetByUserID(ctx, auth.User.ID)
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
		return
	}

	project := &Project{
		Description: projectInfo.Description,
		Name:        projectInfo.Name,
	}

	transaction, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err != nil {
			err = errs.Combine(err, transaction.Rollback())
			return
		}

		err = transaction.Commit()
	}()

	prj, err := transaction.Projects().Insert(ctx, project)
	if err != nil {
		return nil, err
	}

	_, err = transaction.ProjectMembers().Insert(ctx, auth.User.ID, prj.ID)
	if err != nil {
		return nil, err
	}

	return prj, nil
}

// DeleteProject is a method for deleting project by id
func (s *Service) DeleteProject(ctx context.Context, projectID uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return err
	}

	// TODO: before deletion we should check if user is a project member
	return s.store.Projects().Delete(ctx, projectID)
}

// UpdateProject is a method for updating project description by id
func (s *Service) UpdateProject(ctx context.Context, projectID uuid.UUID, description string) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	project, err := s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return nil, errs.New("Project doesn't exist!")
	}

	project.Description = description

	err = s.store.Projects().Update(ctx, project)
	if err != nil {
		return nil, err
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
		return nil, ErrUnauthorized.Wrap(err)
	}

	//var userIDs []uuid.UUID
	var userErr errs.Group

	// collect user querying errors
	for _, email := range emails {
		user, err := s.store.Users().GetByEmail(ctx, email)
		if err != nil {
			userErr.Add(err)
			continue
		}

		users = append(users, user)
		//userIDs = append(userIDs, user.ID)
	}

	if err = userErr.Err(); err != nil {
		return nil, err
	}

	// add project members in transaction scope
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, err
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
			return nil, err
		}
	}

	return users, nil
}

// DeleteProjectMembers removes users by email from given project
func (s *Service) DeleteProjectMembers(ctx context.Context, projectID uuid.UUID, emails []string) (err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	if _, err = s.isProjectMember(ctx, auth.User.ID, projectID); err != nil {
		return ErrUnauthorized.Wrap(err)
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

		userIDs = append(userIDs, user.ID)
	}

	if err = userErr.Err(); err != nil {
		return err
	}

	// delete project members in transaction scope
	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return err
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
			return err
		}
	}

	return nil
}

// GetProjectMembers returns ProjectMembers for given Project
func (s *Service) GetProjectMembers(ctx context.Context, projectID uuid.UUID, pagination Pagination) (pm []ProjectMember, err error) {
	defer mon.Task()(&ctx)(&err)
	_, err = GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	if pagination.Limit > maxLimit {
		pagination.Limit = maxLimit
	}

	return s.store.ProjectMembers().GetByProjectID(ctx, projectID, pagination)
}

// CreateAPIKey creates new api key
func (s *Service) CreateAPIKey(ctx context.Context, projectID uuid.UUID, name string) (*APIKeyInfo, *APIKey, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, nil, ErrUnauthorized.Wrap(err)
	}

	key, err := CreateAPIKey()
	if err != nil {
		return nil, nil, err
	}

	info, err := s.store.APIKeys().Create(ctx, *key, APIKeyInfo{
		Name:      name,
		ProjectID: projectID,
	})
	return info, key, err
}

// GetAPIKeyInfo retrieves api key by id
func (s *Service) GetAPIKeyInfo(ctx context.Context, id uuid.UUID) (*APIKeyInfo, error) {
	var err error
	defer mon.Task()(&ctx)(&err)

	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	key, err := s.store.APIKeys().Get(ctx, id)
	if err != nil {
		return nil, err
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
		return err
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return err
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
			return err
		}
	}

	return nil
}

// GetAPIKeysInfoByProjectID retrieves all api keys for a given project
func (s *Service) GetAPIKeysInfoByProjectID(ctx context.Context, projectID uuid.UUID) (info []APIKeyInfo, err error) {
	defer mon.Task()(&ctx)(&err)
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	_, err = s.isProjectMember(ctx, auth.User.ID, projectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}

	return s.store.APIKeys().GetByProjectID(ctx, projectID)
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

	claims, err := s.authenticate(token)
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

// checkProjectLimit is used to check if user is able to create a new project
// TODO: remove after vanguard release
func (s *Service) checkProjectLimit(ctx context.Context, userID uuid.UUID) error {
	registrationToken, err := s.store.RegistrationTokens().GetByOwnerID(ctx, userID)
	if err != nil {
		return err
	}

	projects, err := s.GetUsersProjects(ctx)
	if err != nil {
		return err
	}
	if len(projects) >= registrationToken.ProjectLimit {
		return errs.New("max project count is %d", registrationToken.ProjectLimit)
	}

	return nil
}

// CreateRegToken creates new registration token. Needed for testing
// TODO: remove after vanguard release
func (s *Service) CreateRegToken(ctx context.Context, projLimit int) (*RegistrationToken, error) {
	return s.store.RegistrationTokens().Create(ctx, projLimit)
}

// createToken creates string representation
func (s *Service) createToken(claims *consoleauth.Claims) (string, error) {
	json, err := claims.JSON()
	if err != nil {
		return "", err
	}

	token := consoleauth.Token{Payload: json}
	err = signToken(&token, s.Signer)
	if err != nil {
		return "", err
	}

	return token.String(), nil
}

// authenticate validates token signature and returns authenticated *satelliteauth.Authorization
func (s *Service) authenticate(token consoleauth.Token) (*consoleauth.Claims, error) {
	signature := token.Signature

	err := signToken(&token, s.Signer)
	if err != nil {
		return nil, err
	}

	if subtle.ConstantTimeCompare(signature, token.Signature) != 1 {
		return nil, errs.New("incorrect signature")
	}

	claims, err := consoleauth.FromJSON(token.Payload)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// authorize checks claims and returns authorized User
func (s *Service) authorize(ctx context.Context, claims *consoleauth.Claims) (*User, error) {
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

// ErrNoMembership is error type of not belonging to a specific project
var ErrNoMembership = errs.Class("no membership error")

// isProjectMember checks if the user is a member of given project
func (s *Service) isProjectMember(ctx context.Context, userID uuid.UUID, projectID uuid.UUID) (result isProjectMember, err error) {
	project, err := s.store.Projects().Get(ctx, projectID)
	if err != nil {
		return
	}

	memberships, err := s.store.ProjectMembers().GetByMemberID(ctx, userID)
	if err != nil {
		return
	}

	for _, membership := range memberships {
		if membership.ProjectID == projectID {
			result.membership = &membership
			result.project = project
			return
		}
	}

	return isProjectMember{}, ErrNoMembership.New("user %s is not a member of project %s", userID, project.ID)
}
