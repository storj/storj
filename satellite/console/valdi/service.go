// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package valdi

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
	"storj.io/storj/satellite/console/valdi/valdiclient"
)

var mon = monkit.Package()

var (
	// Error describes internal valdi error.
	Error = errs.Class("valdi service")

	// ErrPrivateKey is error type of private key.
	ErrPrivateKey = errs.Class("private key")

	// ErrEmail is error type of email.
	ErrEmail = errs.Class("email")

	// ErrCreateAPIKey is error type of create api key.
	ErrCreateAPIKey = errs.Class("create api key")

	// ErrCreateUser is error type of create user.
	ErrCreateUser = errs.Class("create user")
)

// Config contains configurable values for valdi service.
type Config struct {
	SatelliteEmail string `help:"the base email address for valdi satellite project email addresses. Important: once this has been used to create users, it should not be changed" default:""`
	valdiclient.Config
}

// Service handles valdi functionality.
type Service struct {
	log         *zap.Logger
	client      *valdiclient.Client
	emailName   string
	emailDomain string
}

// NewService is a constructor for valdi Service.
func NewService(log *zap.Logger, config Config, client *valdiclient.Client) (*Service, error) {
	if client == nil {
		return nil, Error.New("valdi client cannot be nil")
	}

	if !utils.ValidateEmail(config.SatelliteEmail) {
		return nil, Error.Wrap(ErrEmail.New("invalid satellite valdi email: %s", config.SatelliteEmail))
	}

	emailParts := strings.Split(config.SatelliteEmail, "@")

	return &Service{
		log:         log,
		client:      client,
		emailName:   emailParts[0],
		emailDomain: emailParts[1],
	}, nil
}

// CreateAPIKey creates an API key.
func (s *Service) CreateAPIKey(ctx context.Context, projectID uuid.UUID) (_ *valdiclient.CreateAPIKeyResponse, status int, err error) {
	defer mon.Task()(&ctx)(&err)

	email := s.CreateUserEmail(projectID)

	apiKey, status, err := s.client.CreateAPIKey(ctx, email)
	if err != nil {
		err = Error.Wrap(err)
	}
	return apiKey, status, err
}

// CreateUser creates a user.
func (s *Service) CreateUser(ctx context.Context, projectID uuid.UUID) (status int, err error) {
	defer mon.Task()(&ctx)(&err)

	email := s.CreateUserEmail(projectID)

	var username uuid.UUID
	username, err = uuid.New()
	if err != nil {
		return http.StatusInternalServerError, Error.Wrap(err)
	}

	createUserData := valdiclient.UserCreationData{
		Email:    email,
		Username: username.String(),
		Country:  "USA",
	}

	status, err = s.client.CreateUser(ctx, createUserData)
	if err != nil {
		err = Error.Wrap(err)
	}
	return status, err
}

// CreateUserEmail creates an email address for a valdi user.
func (s *Service) CreateUserEmail(projectID uuid.UUID) string {
	return fmt.Sprintf("%s+%s@%s", s.emailName, projectID.String(), s.emailDomain)
}
