// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import "time"

// PeriodConfig contains the configuration for Period.
type PeriodConfig struct {
	FromHour int `help:"hour to start cleanup" default:"0"`
	ToHour   int `help:"hour to stop cleanup" default:"24"`
}

// Period is an availability check which is false if the current time is outside the configured period.
type Period struct {
	config PeriodConfig
}

// NewPeriod creates a new Period.
func NewPeriod(config PeriodConfig) *Period {
	return &Period{config: config}
}

// Enabled implements Enablement.
func (p *Period) Enabled() (bool, error) {
	hour := time.Now().UTC().Hour()
	return hour >= p.config.FromHour && hour <= p.config.ToHour, nil
}

var _ Enablement = (*Period)(nil)
