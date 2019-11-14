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

var (
	// Error is the default error class for referrals package.
	Error = errs.Class("referrals")

	// ErrReferralsConfigMissing is a error class for reporting missing referrals service configuration
	ErrReferralsConfigMissing = errs.Class("misssing referrals service configuration")
)

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
	dailer rpc.Dialer
}

// NewService returns a service for handling referrals information.
func NewService(log *zap.Logger, signer signing.Signer, config Config, dialer rpc.Dialer) *Service {
	return &Service{
		log:    log,
		signer: signer,
		config: config,
		dailer: dialer,
	}
}

func (service *Service) ReferralManagerConn(ctx context.Context) (*rpc.Conn, error) {
	if service.config.ReferralManagerURL.IsZero() {
		return nil, ErrReferralsConfigMissing.New("")
	}

	conn, err := service.dailer.DialAddressID(ctx, service.config.ReferralManagerURL.Address, service.config.ReferralManagerURL.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return conn, nil
}

func (service *Service) GetTokens(ctx context.Context, client rpc.ReferralManagerClient, userID *uuid.UUID) ([]uuid.UUID, error) {
	if userID == nil {
		return nil, Error.New("invalid argument")
	}

	response, err := client.GetTokens(ctx, &pb.GetTokensRequest{
		UserId: userID[:],
	})
	if err != nil {
		return nil, Error.Wrap(err)
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

// bytesToUUID is used to convert []byte to UUID
func bytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}
