// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/console"
)

var mon = monkit.Package()

var (
	// ErrUsedEmail is an error class for reporting already used emails.
	ErrUsedEmail = errs.Class("email used error")
)

// Config is for referrals service.
type Config struct {
	ReferralManagerURL storj.NodeURL
}

// Service allows communicating with the Referral Manager
//
// architecture: Service
type Service struct {
	log          *zap.Logger
	signer       signing.Signer
	config       Config
	dialer       rpc.Dialer
	db           console.Users
	passwordCost int
}

// NewService returns a service for handling referrals information.
func NewService(log *zap.Logger, signer signing.Signer, config Config, dialer rpc.Dialer, db console.Users, passwordCost int) *Service {
	return &Service{
		log:          log,
		signer:       signer,
		config:       config,
		dialer:       dialer,
		db:           db,
		passwordCost: passwordCost,
	}
}

// GetTokens returns tokens based on user ID.
func (service *Service) GetTokens(ctx context.Context, userID *uuid.UUID) (tokens []uuid.UUID, err error) {
	defer mon.Task()(&ctx)(&err)
	if userID.IsZero() {
		return nil, errs.New("user ID is not defined")
	}

	conn, err := service.referralManagerConn(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	defer func() {
		err = conn.Close()
	}()

	client := pb.NewDRPCReferralManagerClient(conn.Raw())
	response, err := client.GetTokens(ctx, &pb.GetTokensRequest{
		OwnerUserId:      userID[:],
		OwnerSatelliteId: service.signer.ID(),
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	tokensInBytes := response.GetTokenSecrets()
	if tokensInBytes != nil && len(tokensInBytes) == 0 {
		return nil, errs.New("no available tokens")
	}

	tokens = make([]uuid.UUID, len(tokensInBytes))
	for i := range tokensInBytes {
		token, err := dbutil.BytesToUUID(tokensInBytes[i])
		if err != nil {
			service.log.Debug("failed to convert bytes to UUID", zap.Error(err))
			continue
		}
		tokens[i] = token
	}

	return tokens, nil
}

// CreateUser validates user's registration information and creates a new user.
func (service *Service) CreateUser(ctx context.Context, user CreateUser) (_ *console.User, err error) {
	defer mon.Task()(&ctx)(&err)
	if err := user.IsValid(); err != nil {
		return nil, ErrValidation.Wrap(err)
	}

	if len(user.ReferralToken) == 0 {
		return nil, errs.New("referral token is not defined")
	}

	_, err = service.db.GetByEmail(ctx, user.Email)
	if err == nil {
		return nil, ErrUsedEmail.New("")
	}

	userID, err := uuid.New()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	err = service.redeemToken(ctx, userID, user.ReferralToken)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), service.passwordCost)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	newUser := &console.User{
		ID:           *userID,
		Email:        user.Email,
		FullName:     user.FullName,
		ShortName:    user.ShortName,
		PasswordHash: hash,
	}

	u, err := service.db.Insert(ctx,
		newUser,
	)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return u, nil
}

func (service *Service) redeemToken(ctx context.Context, userID *uuid.UUID, token string) error {
	conn, err := service.referralManagerConn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		err = conn.Close()
	}()

	if userID.IsZero() || len(token) == 0 {
		return errs.New("invalid argument")
	}

	referralToken, err := uuid.Parse(token)
	if err != nil {
		return errs.Wrap(err)
	}

	client := pb.NewDRPCReferralManagerClient(conn.Raw())
	_, err = client.RedeemToken(ctx, &pb.RedeemTokenRequest{
		Token:             referralToken[:],
		RedeemUserId:      userID[:],
		RedeemSatelliteId: service.signer.ID(),
	})
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func (service *Service) referralManagerConn(ctx context.Context) (*rpc.Conn, error) {
	if service.config.ReferralManagerURL.IsZero() {
		return nil, errs.New("missing referral manager url configuration")
	}

	return service.dialer.DialAddressID(ctx, service.config.ReferralManagerURL.Address, service.config.ReferralManagerURL.ID)
}
