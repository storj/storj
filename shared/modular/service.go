// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

// Service is a marker annotation for all components which should be started (with all the dependencies).
type Service struct{}

func (s Service) String() string {
	return "Service"
}
