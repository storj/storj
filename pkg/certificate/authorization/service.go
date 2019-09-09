// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/storj/pkg/pb"
)

// ErrService is the default error class for the authorization service.
var ErrService = errs.Class("authorization service error")

// Service is the authorization service.
type Service struct {
	log *zap.Logger
	db  *DB
}

// NewService creates a new authorization service.
func NewService(log *zap.Logger, db *DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// GetOrCreate will return an authorization for the given user ID
func (service *Service) GetOrCreate(ctx context.Context, req *pb.AuthorizationRequest) (_ *pb.AuthorizationResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	existingGroup, err := service.db.Get(ctx, req.UserId)
	if err != nil {
		msg := "error getting authorizations"
		err = ErrService.Wrap(err)
		service.log.Error(msg, zap.Error(err))
		return nil, status.Error(codes.Internal, msg)
	}

	if len(existingGroup) > 0 {
		authorization := existingGroup[0]
		return &pb.AuthorizationResponse{
			Token: authorization.Token.String(),
		}, nil
	}

	createdGroup, err := service.db.Create(ctx, req.UserId, 1)
	if err != nil {
		msg := "error creating authorization"
		err = ErrService.Wrap(err)
		service.log.Error(msg, zap.Error(err))

		switch err {
		case ErrCount, ErrEmptyUserID:
			return nil, status.Error(codes.InvalidArgument, msg)
		default:
			return nil, status.Error(codes.Internal, msg)
		}
	}

	groupLen := len(createdGroup)
	if groupLen != 1 {
		clientMsg := "error creating authorization"
		internalMsg := clientMsg + fmt.Sprintf("; expected 1, got %d", groupLen)

		service.log.Error(internalMsg)
		return nil, status.Error(codes.Internal, ErrEndpoint.New(clientMsg).Error())
	}

	authorization := createdGroup[0]

	return &pb.AuthorizationResponse{
		Token: authorization.Token.String(),
	}, nil
}
