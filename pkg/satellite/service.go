// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"go.uber.org/zap"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/satellite/satelliteauth"
)

// Service is handling accounts related logic
type Service struct {
	Signer

	store DB
	log   *zap.Logger
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, signer Signer, store DB) (*Service, error) {
	if signer == nil {
		return nil, errs.New("signer can't be nil")
	}

	if store == nil {
		return nil, errs.New("store can't be nil")
	}

	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	return &Service{Signer: signer, store: store, log: log}, nil
}

// CreateUser gets password hash value and creates new User
func (s *Service) CreateUser(ctx context.Context, userInfo UserInfo, companyInfo CompanyInfo) (*User, error) {
	passwordHash := sha256.Sum256([]byte(userInfo.Password))

	if err := userInfo.IsValid(); err != nil {
		return nil, err
	}

	//TODO(yar): separate creation of user and company
	user, err := s.store.Users().Insert(ctx, &User{
		Email:        userInfo.Email,
		FirstName:    userInfo.FirstName,
		LastName:     userInfo.LastName,
		PasswordHash: passwordHash[:],
	})

	if err != nil {
		return nil, err
	}

	_, err = s.store.Companies().Insert(ctx, &Company{
		UserID:     user.ID,
		Name:       companyInfo.Name,
		Address:    companyInfo.Address,
		Country:    companyInfo.Country,
		City:       companyInfo.City,
		State:      companyInfo.State,
		PostalCode: companyInfo.PostalCode,
	})

	if err != nil {
		s.log.Error(err.Error())
	}

	return user, nil
}

// CreateCompany creates Company for authorized User
func (s *Service) CreateCompany(ctx context.Context, info CompanyInfo) (*Company, error) {
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Companies().Insert(ctx, &Company{
		UserID:     auth.User.ID,
		Name:       info.Name,
		Address:    info.Address,
		Country:    info.Country,
		City:       info.City,
		State:      info.State,
		PostalCode: info.PostalCode,
	})
}

// Token authenticates User by credentials and returns auth token
func (s *Service) Token(ctx context.Context, email, password string) (string, error) {
	passwordHash := sha256.Sum256([]byte(password))

	user, err := s.store.Users().GetByCredentials(ctx, passwordHash[:], email)
	if err != nil {
		return "", err
	}

	//TODO: move expiration time to constants
	claims := satelliteauth.Claims{
		ID:         user.ID,
		Expiration: time.Now().Add(time.Minute * 15),
	}

	token, err := s.createToken(&claims)
	if err != nil {
		return "", err
	}

	return token, nil
}

// GetUser returns User by id
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	_, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Users().Get(ctx, id)
}

// UpdateUser updates User with given id
func (s *Service) UpdateUser(ctx context.Context, id uuid.UUID, info UserInfo) error {
	_, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	if err = info.IsValid(); err != nil {
		return err
	}

	//TODO(yar): remove when validation is added
	var passwordHash []byte
	if info.Password != "" {
		hash := sha256.Sum256([]byte(info.Password))
		passwordHash = hash[:]
	}

	return s.store.Users().Update(ctx, &User{
		ID:           id,
		FirstName:    info.FirstName,
		LastName:     info.LastName,
		Email:        info.Email,
		PasswordHash: passwordHash,
	})
}

// DeleteUser deletes User by id
func (s *Service) DeleteUser(ctx context.Context, id uuid.UUID) error {
	_, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return s.store.Users().Delete(ctx, id)
}

// GetCompany returns Company by userID
func (s *Service) GetCompany(ctx context.Context, userID uuid.UUID) (*Company, error) {
	_, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Companies().GetByUserID(ctx, userID)
}

// UpdateCompany updates Company with given userID
func (s *Service) UpdateCompany(ctx context.Context, userID uuid.UUID, info CompanyInfo) error {
	_, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return s.store.Companies().Update(ctx, &Company{
		UserID:     userID,
		Name:       info.Name,
		Address:    info.Address,
		Country:    info.Country,
		City:       info.City,
		State:      info.State,
		PostalCode: info.PostalCode,
	})
}

// GetProject is a method for querying project by id
func (s *Service) GetProject(ctx context.Context, projectID uuid.UUID) (*Project, error) {
	_, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Projects().Get(ctx, projectID)
}

// GetUsersProjects is a method for querying all projects
func (s *Service) GetUsersProjects(ctx context.Context) ([]Project, error) {
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Projects().GetByUserID(ctx, auth.User.ID)
}

// CreateProject is a method for creating new project
func (s *Service) CreateProject(ctx context.Context, projectInfo ProjectInfo) (*Project, error) {
	auth, err := GetAuth(ctx)
	if err != nil {
		return nil, err
	}

	if !projectInfo.IsTermsAccepted {
		return nil, errs.New("Terms of use should be accepted!")
	}

	project := &Project{
		OwnerID:       &auth.User.ID,
		Description:   projectInfo.Description,
		CompanyName:   projectInfo.CompanyName,
		Name:          projectInfo.Name,
		TermsAccepted: 1, //TODO: get lat version of Term of Use
	}

	// For now we make this operations sequentially.
	// But soon we will add functionality to be able to
	// do any operations in transaction scope
	prj, err := s.store.Projects().Insert(ctx, project)
	if err != nil {
		return nil, err
	}

	// Project owner is also a project member
	_, err = s.store.ProjectMembers().Insert(ctx, auth.User.ID, prj.ID)

	return prj, err
}

// DeleteProject is a method for deleting project by id
func (s *Service) DeleteProject(ctx context.Context, projectID uuid.UUID) error {
	_, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	return s.store.Projects().Delete(ctx, projectID)
}

// UpdateProject is a method for updating project description by id
func (s *Service) UpdateProject(ctx context.Context, projectID uuid.UUID, description string) (*Project, error) {
	_, err := GetAuth(ctx)
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

// AddProjectMember adds User as member of given Project
func (s *Service) AddProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	_, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	_, err = s.store.ProjectMembers().Insert(ctx, userID, projectID)
	return err
}

// DeleteProjectMember removes user membership for given project
func (s *Service) DeleteProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	_, err := GetAuth(ctx)
	if err != nil {
		return err
	}

	// TODO: remove when appropriate method is added
	projects, err := s.store.ProjectMembers().GetAll(ctx)
	if err != nil {
		return err
	}

	for _, project := range projects {
		if project.ProjectID == projectID && project.MemberID == userID {
			return s.store.ProjectMembers().Delete(ctx, project.ID)
		}
	}

	return errs.New("project with id %s doesn't have a member with id %s", projectID.String(), userID.String())
}

// Authorize validates token from context and returns authorized Authorization
func (s *Service) Authorize(ctx context.Context) (Authorization, error) {
	tokenS, ok := auth.GetAPIKey(ctx)
	if !ok {
		return Authorization{}, errs.New("no api key was provided")
	}

	token, err := satelliteauth.FromBase64URLString(string(tokenS))
	if err != nil {
		return Authorization{}, err
	}

	claims, err := s.authenticate(token)
	if err != nil {
		return Authorization{}, err
	}

	user, err := s.authorize(ctx, claims)
	if err != nil {
		return Authorization{}, err
	}

	return Authorization{
		User:   *user,
		Claims: *claims,
	}, nil
}

// createToken creates string representation
func (s *Service) createToken(claims *satelliteauth.Claims) (string, error) {
	json, err := claims.JSON()
	if err != nil {
		return "", err
	}

	token := satelliteauth.Token{Payload: json}
	err = signToken(&token, s.Signer)
	if err != nil {
		return "", err
	}

	return token.String(), nil
}

// authenticate validates token signature and returns authenticated *satelliteauth.Authorization
func (s *Service) authenticate(token satelliteauth.Token) (*satelliteauth.Claims, error) {
	signature := token.Signature

	err := signToken(&token, s.Signer)
	if err != nil {
		return nil, err
	}

	if subtle.ConstantTimeCompare(signature, token.Signature) != 1 {
		return nil, errs.New("incorrect signature")
	}

	claims, err := satelliteauth.FromJSON(token.Payload)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

// authorize checks claims and returns authorized User
func (s *Service) authorize(ctx context.Context, claims *satelliteauth.Claims) (*User, error) {
	if !claims.Expiration.IsZero() && claims.Expiration.Before(time.Now()) {
		return nil, errs.New("token is outdated")
	}

	user, err := s.store.Users().Get(ctx, claims.ID)
	if err != nil {
		return nil, errs.New("authorization failed. no user with id: %s", claims.ID.String())
	}

	return user, nil
}
