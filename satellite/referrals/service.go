// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals

import (
	"context"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/signing"
	"storj.io/storj/pkg/storj"
)

// ErrReferralsConfigMissing is a error class for reporting missing referrals service configuration.
var ErrReferralsConfigMissing = errs.Class("misssing referrals service configuration")

type Config struct {
	ReferralManagerURL storj.NodeURL
}

// ReferralsService allows communicating with the Referral Manager
//
// architecture: Service
type Service struct {
	log    *zap.Logger
	signer signing.Signer
	config Config
	dialer rpc.Dialer
}

// NewService returns a service for handling referrals information.
func NewService(log *zap.Logger, signer signing.Signer, config Config, dialer rpc.Dialer) *Service {
	return &Service{
		log:    log,
		signer: signer,
		config: config,
		dialer: dialer,
	}
}

func (service *Service) GetTokens(ctx context.Context, userID *uuid.UUID) ([]uuid.UUID, error) {
	if userID.IsZero() {
		return nil, errs.New("invalid argument")
	}

	conn, err := service.referralManagerConn(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	defer conn.Close()

	client := conn.ReferralManagerClient()
	response, err := client.GetTokens(ctx, &pb.GetTokensRequest{
		UserId: userID[:],
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	tokensInBytes := response.GetToken()
	var tokens = make([]uuid.UUID, len(tokensInBytes))
	for i := range tokensInBytes {
		token, err := bytesToUUID(tokensInBytes[i])
		if err != nil {
			continue
		}
		tokens[i] = token
	}

	return tokens, nil
}

func (service *Service) RedeemToken(ctx context.Context, userID *uuid.UUID, token string) error {
	conn, err := service.referralManagerConn(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	defer conn.Close()

	if userID.IsZero() || len(token) == 0 {
		return errs.New("invalid argument")
	}

	referralToken, err := uuid.Parse(token)
	if err != nil {
		return errs.Wrap(err)
	}

	client := conn.ReferralManagerClient()
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
		return nil, ErrReferralsConfigMissing.New("")
	}

	return service.dialer.DialAddressID(ctx, service.config.ReferralManagerURL.Address, service.config.ReferralManagerURL.ID)
}

// bytesToUUID is used to convert []byte to UUID
func bytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}
