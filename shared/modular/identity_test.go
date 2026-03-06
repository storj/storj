// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/peertls"
	"storj.io/common/pkcrypto"
)

func TestIdentityConfig_LoadFromPEM(t *testing.T) {
	fi, err := testidentity.NewTestIdentity(t.Context())
	require.NoError(t, err)

	certPEM, err := peertls.ChainBytes(fi.Chain()...)
	require.NoError(t, err)

	keyPEM, err := pkcrypto.PrivateKeyToPEM(fi.Key)
	require.NoError(t, err)

	cfg := IdentityConfig{
		Cert: string(certPEM),
		Key:  string(keyPEM),
	}

	loaded, err := cfg.Load()
	require.NoError(t, err)
	require.Equal(t, fi.ID, loaded.ID)
}

func TestIdentityConfig_FallbackToPath(t *testing.T) {
	// When Cert and Key are empty, Load should fall back to Config.Load
	// which will fail since we don't have valid paths, but that's the expected behavior.
	cfg := IdentityConfig{
		Config: identity.Config{
			CertPath: "/nonexistent/cert.pem",
			KeyPath:  "/nonexistent/key.pem",
		},
	}

	_, err := cfg.Load()
	require.Error(t, err)
}
