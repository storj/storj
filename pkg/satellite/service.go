// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"time"

	"go.uber.org/zap"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

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

// CreateUser gets password hash value and creates new user
func (s *Service) CreateUser(ctx context.Context, userInfo UserInfo, companyInfo CompanyInfo) (*User, error) {
	passwordHash := sha256.Sum256([]byte(userInfo.Password))

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
	user, err := s.Authorize(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Companies().Insert(ctx, &Company{
		UserID:     user.ID,
		Name:       info.Name,
		Address:    info.Address,
		Country:    info.Country,
		City:       info.City,
		State:      info.State,
		PostalCode: info.PostalCode,
	})
}

// Token authenticates user by credentials and returns auth token
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

// GetUser returns user by id
func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (*User, error) {
	_, err := s.Authorize(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Users().Get(ctx, id)
}

// GetCompany returns company by userID
func (s *Service) GetCompany(ctx context.Context, userID uuid.UUID) (*Company, error) {
	_, err := s.Authorize(ctx)
	if err != nil {
		return nil, err
	}

	return s.store.Companies().GetByUserID(ctx, userID)
}

// Authorize validates token from context and returns authenticated and authorized User
func (s *Service) Authorize(ctx context.Context) (*User, error) {
	token, ok := auth.GetAPIKey(ctx)
	if !ok {
		return nil, errs.New("no api key was provided")
	}

	claims, err := s.authenticate(string(token))
	if err != nil {
		return nil, err
	}

	return s.authorize(ctx, claims)
}

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

// authenticate validates token signature and returns authenticated *satelliteauth.Claims
func (s *Service) authenticate(tokenS string) (*satelliteauth.Claims, error) {
	token, err := satelliteauth.FromBase64URLString(tokenS)
	if err != nil {
		return nil, err
	}

	signature := token.Signature

	err = signToken(&token, s.Signer)
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
