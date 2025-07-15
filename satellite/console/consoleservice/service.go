// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleservice

import (
	"context"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/http/requestid"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

var mon = monkit.Package()

// Error describes internal console service error.
var Error = errs.Class("console service")

// ServiceDependencies contains all dependencies that Service needs to operate.
type ServiceDependencies struct {
	ConsoleDB            console.DB
	AccountFreezeService *console.AccountFreezeService
}

// InternalConfig contains internal configuration for the console service.s
type InternalConfig struct {
	varPartners map[string]struct{}
}

// Service is handling accounts related logic.
//
// architecture: Service
type Service struct {
	log, auditLogger *zap.Logger

	deps ServiceDependencies

	config   console.Config
	internal InternalConfig
}

// NewService returns new instance of Service.
func NewService(log *zap.Logger, deps ServiceDependencies, config console.Config) (*Service, error) {
	if deps.ConsoleDB == nil {
		return nil, errs.New("store can't be nil")
	}
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	partners := make(map[string]struct{}, len(config.VarPartners))
	for _, partner := range config.VarPartners {
		partners[partner] = struct{}{}
	}

	return &Service{
		log:         log,
		auditLogger: log.Named("auditlog"),
		deps:        deps,
		config:      config,
		internal:    InternalConfig{varPartners: partners},
	}, nil
}

// Users returns Users struct that contains all users related functionality.
func (s *Service) Users() Users {
	return Users{service: s}
}

// auditLog records console activity with user details and request metadata for auditing purposes.
func (s *Service) auditLog(ctx context.Context, operation string, userID *uuid.UUID, email string, extra ...zap.Field) {
	sourceIP, forwardedForIP := getRequestingIP(ctx)
	fields := append(
		make([]zap.Field, 0, len(extra)+6),
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
	if requestID := requestid.FromContext(ctx); requestID != "" {
		fields = append(fields, zap.String("requestID", requestID))
	}

	fields = append(fields, extra...)
	s.auditLogger.Info("console activity", fields...)
}

// getRequestingIP retrieves the source IP and forwarded-for IP from the context.
func getRequestingIP(ctx context.Context) (source, forwardedFor string) {
	if req := console.GetRequest(ctx); req != nil {
		return req.RemoteAddr, req.Header.Get("X-Forwarded-For")
	}
	return "", ""
}
