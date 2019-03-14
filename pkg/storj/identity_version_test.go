// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

func TestLatestVersion(t *testing.T) {
	version := storj.LatestIDVersion()
	assert.Equal(t, storj.V2, version.Number)
}

func TestIDVersionFromCert(t *testing.T) {
	for versionNumber := range storj.IDVersions {
		_, chain, err := testpeertls.NewVersionedCertChain(1, versionNumber)
		require.NoError(t, err)

		cert := chain[0]

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
		{"multiple versions", storj.V2, "1,2"},
		{"end of version range", storj.V2, "1-2"},
		{"beginning of version range", storj.V1, "1-2"},
		{"middle of version range", storj.V2, "1-3"},
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
		{"single version", storj.V2, "1"},
		{"multiple versions", storj.V2, "1,3"},
		{"version range", storj.V1, "2-3"},
		{"latest alias", storj.V1, "latest"},
		{"malformed PeerIDVersions", storj.V1, "1-"},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		err := storj.IDVersionInVersions(testcase.versionNumber, testcase.versionsStr)
		assert.Error(t, err)
	}
}

func TestIDVersionExtensionHandler_success(t *testing.T) {
	_, identityV1Chain, err := testpeertls.NewVersionedCertChain(2, storj.V1)
	assert.NoError(t, err)

	_, identityV2Chain, err := testpeertls.NewVersionedCertChain(2, storj.V2)
	assert.NoError(t, err)

	latestVersionChain := identityV2Chain
	//alphaVersionChain := identityV2Chain

	testcases := []struct {
		name     string
		versions string
		chain    []*x509.Certificate
	}{
		{"V1", "1", identityV1Chain},
		{"V2", "2", identityV2Chain},
		{"V1 & V2 with V2 chain", "1,2", identityV2Chain},
		{"V1 & V2 with V1 chain", "1,2", identityV1Chain},
		{"latest version", "latest", latestVersionChain},
		//{"alpha version", "alpha", alphaVersionChain},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		opts := &extensions.Options{PeerIDVersions: testcase.versions}

		ext := testcase.chain[peertls.CAIndex].ExtraExtensions[0]
		err := storj.IDVersionHandler.NewHandlerFunc(opts)(ext, identity.ToChains(testcase.chain))
		assert.NoError(t, err)

		extensionMap := tlsopts.NewExtensionsMap(testcase.chain...)
		handlerFuncMap := extensions.AllHandlers.WithOptions(opts)

		err = extensionMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.chain)) // nil, identity.ToChains(testcase.chain))
		assert.NoError(t, err)
	}
}

func TestIDVersionExtensionHandler_error(t *testing.T) {
	_, identityV1Chain, err := testpeertls.NewVersionedCertChain(2, storj.V1)
	assert.NoError(t, err)

	_, identityV2Chain, err := testpeertls.NewVersionedCertChain(2, storj.V2)
	assert.NoError(t, err)

	testcases := []struct {
		name     string
		versions string
		chain    []*x509.Certificate
	}{
		{"single version mismatch", "1", identityV2Chain},
		{"multiple versions mismatch", "1,3", identityV2Chain},
		{"latest version", "latest", identityV1Chain},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		opts := &extensions.Options{PeerIDVersions: testcase.versions}

		ext := testcase.chain[peertls.CAIndex].ExtraExtensions[0]
		err := storj.IDVersionHandler.NewHandlerFunc(opts)(ext, identity.ToChains(testcase.chain))
		assert.Error(t, err)

		extensionMap := tlsopts.NewExtensionsMap(testcase.chain...)
		handlerFuncMap := extensions.AllHandlers.WithOptions(opts)

		err = extensionMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.chain)) // nil, identity.ToChains(testcase.chain))
		assert.Error(t, err)
	}
}

func TestAddVersionExt(t *testing.T) {
	for versionNumber := range storj.IDVersions {
		_, chain, err := testpeertls.NewCertChain(1)
		require.NoError(t, err)
		cert := chain[0]

		err = storj.AddVersionExt(versionNumber, cert)
		assert.NoError(t, err)

		versionExt := new(pkix.Extension)
		for _, ext := range cert.ExtraExtensions {
			if extensions.IdentityVersionExtID.ToASN1().Equal(ext.Id) {
				*versionExt = ext
				break
			}
		}
		require.NotEmpty(t, versionExt)
		assert.Equal(t, versionExt.Value, []byte{byte(versionNumber)})
	}
}
