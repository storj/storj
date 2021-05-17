// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package planneddowntime

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// Service is the planned downtime service.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	db DB
}

// NewService creates new instance of service.
func NewService(log *zap.Logger, db DB) *Service {
	return &Service{
		log: log,
		db:  db,
	}
}

// Add verifies a provided planned downtime on the satellite, then adds it to the local db.
func (s *Service) Add(ctx context.Context, planned Entry) error {
	// TODO satellite validation

	return s.db.Add(ctx, planned)
}

// GetScheduled gets a list of ongoing and upcoming planned downtime entries from the local db.
func (s *Service) GetScheduled(ctx context.Context, since time.Time) ([]Entry, error) {
	return s.db.GetScheduled(ctx, since)
}

// GetCompleted gets a list of completed planned downtime entries from the local db.
func (s *Service) GetCompleted(ctx context.Context, before time.Time) ([]Entry, error) {
	return s.db.GetScheduled(ctx, before)
}

// Delete deletes a planned downtime entry from the satellite, then from the local db.
func (s *Service) Delete(ctx context.Context, id []byte) error {
	// TODO delete from satellite

	return s.db.Delete(ctx, id)
}
