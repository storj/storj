// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"context"
	"time"

	"github.com/zeebo/errs"
)

var (
	// ErrSetup is returned when there's an error with setup
	ErrSetup = errs.Class("setup error")
)

// SetupIdentity ensures a CA and identity exist
func SetupIdentity(ctx context.Context, c CASetupConfig, i IdentitySetupConfig) error {
	if s := c.Status(); s != NoCertNoKey && !c.Overwrite {
		return ErrSetup.New("certificate authority file(s) exist: %s", s)
	}

	t, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return errs.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	// Create a new certificate authority
	ca, err := c.Create(ctx)
	if err != nil {
		return err
	}

	if s := i.Status(); s != NoCertNoKey && !i.Overwrite {
		return ErrSetup.New("identity file(s) exist: %s", s)
	}

	// Create identity from new CA
	_, err = i.Create(ca)
	return err
}

// SetupCA ensures a CA exists
func SetupCA(ctx context.Context, c CASetupConfig) error {
	if s := c.Status(); s != NoCertNoKey && !c.Overwrite {
		return ErrSetup.New("certificate authority file(s) exist: %s", s)
	}

	t, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return errs.Wrap(err)
	}
	ctx, cancel := context.WithTimeout(ctx, t)
	defer cancel()

	// Create a new certificate authority
	_, err = c.Create(ctx)
	return err
}
