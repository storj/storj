// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// N.B. this file exists to ease a migration from cmd/uplinkng to
// cmd/uplink. There is a test that imports and uses this function
// and cmd/uplinkng does not yet use it the same way.

package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/uplink"
	"storj.io/uplink/edge"
)

// RegisterAccess registers an access grant with a Gateway Authorization Service.
func RegisterAccess(ctx context.Context, access *uplink.Access, authService string, public bool, certificateFile string) (credentials *edge.Credentials, err error) {
	if authService == "" {
		return nil, errs.New("no auth service address provided")
	}

	// preserve compatibility with previous https service
	authService = strings.TrimPrefix(authService, "https://")
	authService = strings.TrimSuffix(authService, "/")
	if !strings.Contains(authService, ":") {
		authService += ":7777"
	}

	var certificatePEM []byte
	if certificateFile != "" {
		certificatePEM, err = os.ReadFile(certificateFile)
		if err != nil {
			return nil, errs.New("can't read certificate file: %w", err)
		}
	}

	edgeConfig := edge.Config{
		AuthServiceAddress: authService,
		CertificatePEM:     certificatePEM,
	}
	return edgeConfig.RegisterAccess(ctx, access, &edge.RegisterAccessOptions{Public: public})
}
