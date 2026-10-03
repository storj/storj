// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package opentelemetry

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/resource"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/version"
	"storj.io/storj/shared/modular"
)

func TestResource(t *testing.T) {
	ctx := context.Background()

	semVer, err := version.NewSemVer("v1.158.2")
	require.NoError(t, err)

	res, err := newResource(ctx, Config{
		Service:    "storagenode",
		Namespace:  "storj",
		InstanceID: "storagenode1",
	}, version.Info{Version: semVer})
	require.NoError(t, err)

	attrs := resourceAttributes(res)
	require.Equal(t, "storj", attrs["service.namespace"])
	require.Equal(t, "storagenode", attrs["service.name"])
	require.Equal(t, "storagenode1", attrs["service.instance.id"])
	require.Equal(t, "v1.158.2", attrs["service.version"])
	require.NotEmpty(t, attrs["host.name"])
}

func TestResourceEmptyValues(t *testing.T) {
	ctx := context.Background()

	res, err := newResource(ctx, Config{}, version.Info{})
	require.NoError(t, err)

	attrs := resourceAttributes(res)
	require.NotContains(t, attrs, "service.namespace")
	require.NotContains(t, attrs, "service.instance.id")
	require.NotContains(t, attrs, "service.version")
	require.NotContains(t, attrs, "service.name")
}

func TestNodeIDOf(t *testing.T) {
	fullIdentity := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())

	dir := t.TempDir()
	identityCfg := identity.Config{
		CertPath: filepath.Join(dir, "identity.cert"),
		KeyPath:  filepath.Join(dir, "identity.key"),
	}
	require.NoError(t, identityCfg.Save(fullIdentity))

	expected := fullIdentity.ID.String()

	t.Run("cert path", func(t *testing.T) {
		require.Equal(t, expected, nodeIDOf(modular.IdentityConfig{Config: identityCfg}))
	})

	t.Run("cert content", func(t *testing.T) {
		cert, err := os.ReadFile(identityCfg.CertPath)
		require.NoError(t, err)
		require.Equal(t, expected, nodeIDOf(modular.IdentityConfig{Cert: string(cert)}))
	})

	t.Run("missing identity", func(t *testing.T) {
		require.Empty(t, nodeIDOf(modular.IdentityConfig{Config: identity.Config{
			CertPath: filepath.Join(dir, "missing.cert"),
		}}))
		require.Empty(t, nodeIDOf(modular.IdentityConfig{}))
	})
}

func resourceAttributes(res *resource.Resource) map[string]string {
	attrs := map[string]string{}
	for _, kv := range res.Attributes() {
		attrs[string(kv.Key)] = kv.Value.String()
	}
	return attrs
}
