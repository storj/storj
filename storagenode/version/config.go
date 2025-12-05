// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"github.com/zeebo/errs"

	"storj.io/storj/private/version/checker"
)

// Config is the config for the Storagenode Version Checker.
type Config struct {
	checker.Config

	RunMode Mode `help:"Define the run mode for the version checker. Options (once,periodic,disable)" default:"periodic"`
}

// Mode is the mode to run the version checker in.
type Mode string

const (
	checkerModeOnce     Mode = "once"     // run the version checker once
	checkerModePeriodic Mode = "periodic" // run the version checker periodically
	checkerModeDisable  Mode = "disable"  // disable the version checker
)

// String implements pflag.Value.
func (m *Mode) String() string {
	return string(*m)
}

// Set implements pflag.Value.
func (m *Mode) Set(s string) error {
	if s == "" {
		return errs.New("checker mode cannot be empty")
	}

	mode, err := ParseCheckerMode(s)
	if err != nil {
		return err
	}

	*m = mode
	return nil
}

// Type implements pflag.Value.
func (m *Mode) Type() string {
	return "run-mode"
}

// Disabled returns true if the checker is disabled.
func (m *Mode) Disabled() bool {
	return *m == checkerModeDisable
}

// Periodic returns true if the checker is periodic.
func (m *Mode) Periodic() bool {
	return *m == checkerModePeriodic
}

// Once returns true if the checker is once.
func (m *Mode) Once() bool {
	return *m == checkerModeOnce
}

// ParseCheckerMode parses the string representation of the CheckerMode.
func ParseCheckerMode(s string) (Mode, error) {
	switch Mode(s) {
	case checkerModeOnce, checkerModePeriodic, checkerModeDisable:
		return Mode(s), nil
	default:
		return "", errs.New("invalid checker run mode %q", s)
	}
}
