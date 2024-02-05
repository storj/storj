// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package emission

// Service is an emission service.
// Performs emissions impact calculations.
//
// architecture: Service
type Service struct {
	config *Config
}

// NewService creates a new ImpactCalculator with the given configuration.
func NewService(config *Config) *Service {
	return &Service{config: config}
}
