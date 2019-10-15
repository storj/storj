// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Package partners implements partners management for attributions.
package partners

import (
	"bytes"
	"context"
	htmltemplate "html/template"
	"path/filepath"
	"sync"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/post"
)

var (
	// Error is the default error class for partners package.
	Error = errs.Class("partners error class")

	mon = monkit.Package()
)

// Partner contains information about a partner.
type Partner struct {
	Name string
	ID   string
}

// UserAgent returns partners cano user agent.
func (p *Partner) UserAgent() string { return p.Name }

// CanonicalUserAgent returns canonicalizes the user name, which is suitable for lookups.
func CanonicalUserAgent(useragent string) string { return strigns.ToLower(useragent)}

// DB allows access to partners database.
type DB interface {
	ByName(ctx context.Context, name string) ([]Partner, error)
	ByID(ctx context.Context, id string) (Partner, error)
	ByUserAgent(ctx context.Context, agent string) (Partner, error)
}

// Service allows manipulating and accessing partner information.
type Service struct {
	log *zap.Logger
	db  DB
}

// NewService returns a service for handling partner information.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}
