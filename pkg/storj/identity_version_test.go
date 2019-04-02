// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"crypto/x509"
	"testing"

	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

func TestLatestVersion(t *testing.T) {
	version := storj.LatestIDVersion()
	assert.Equal(t, storj.V1, version.Number)
}

func TestIDVersionFromCert(t *testing.T) {
	for versionNumber := range storj.IDVersions {
		t.Logf("id version %d", versionNumber)
		_, chain, err := testpeertls.NewCertChain(2, versionNumber)
		require.NoError(t, err)

		cert := chain[peertls.CAIndex]

		version, err := storj.IDVersionFromCert(cert)
		require.NoError(t, err)

		assert.Equal(t, versionNumber, version.Number)
	}
}

func TestIDVersionInVersions_match(t *testing.T) {
	testcases := []struct {
		name          string
		versionNumber storj.IDVersionNumber
		versionsStr   string
	}{
		{"single version", storj.V1, "1"},
		{"beginning of version range", storj.V1, "1-2"},
		{"latest alias", storj.LatestIDVersion().Number, "latest"},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		err := storj.IDVersionInVersions(testcase.versionNumber, testcase.versionsStr)
		assert.NoError(t, err)
	}
}

func TestIDVersionInVersions_error(t *testing.T) {
	testcases := []struct {
		name          string
		versionNumber storj.IDVersionNumber
		versionsStr   string
	}{
		{"version range", storj.V1, "2-3"},
		{"malformed PeerIDVersions", storj.V1, "1-"},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		err := storj.IDVersionInVersions(testcase.versionNumber, testcase.versionsStr)
		assert.Error(t, err)
	}
}

func TestIDVersionExtensionHandler_success(t *testing.T) {
	_, identityV1Chain, err := testpeertls.NewCertChain(2, storj.V1)
	assert.NoError(t, err)

	latestVersionChain := identityV1Chain

	testcases := []struct {
		name     string
		versions string
		chain    []*x509.Certificate
	}{
		{"V1", "1", identityV1Chain},
		{"V1 & V2 with V1 chain", "1,2", identityV1Chain},
		{"latest version", "latest", latestVersionChain},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		opts := &extensions.Options{PeerIDVersions: testcase.versions}

		cert := testcase.chain[peertls.CAIndex]
		ext := cert.Extensions[len(cert.Extensions)-1]
		err := storj.IDVersionHandler.NewHandlerFunc(opts)(ext, identity.ToChains(testcase.chain))
		assert.NoError(t, err)

		extensionMap := tlsopts.NewExtensionsMap(testcase.chain...)
		handlerFuncMap := extensions.AllHandlers.WithOptions(opts)

		err = extensionMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.chain))
		assert.NoError(t, err)
	}
}
