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
		{"single version", storj.V0, "0"},
		{"first versions", storj.V0, "0,1"},
		{"middle versions", storj.V1, "0,1,2"},
		{"last versions", storj.V1, "k,1"},
		{"end of version range", storj.V1, "0-1"},
		{"beginning of version range", storj.V0, "0-1"},
		{"middle of version range", storj.V1, "0-2"},
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
		{"single version", storj.V1, "0"},
		{"first versions", storj.V1, "0,2"},
		{"middle versions", storj.V1, "0,1,2"},
		{"last versions", storj.V1, "k,1"},
		{"version range", storj.V0, "0-1"},
		{"latest alias", storj.V1, "latest"},
		{"malformed PeerIDVersions", storj.V0, "0-"},
	}

	for _, testcase := range testcases {
		t.Log(testcase.name)
		err := storj.IDVersionInVersions(testcase.versionNumber, testcase.versionsStr)
		assert.Error(t, err)
	}
}

func TestIDVersionExtensionHandler_success(t *testing.T) {
	_, identityV0Chain, err := testpeertls.NewCertChain(2, storj.V0)
	assert.NoError(t, err)

	_, identityV1Chain, err := testpeertls.NewCertChain(2, storj.V1)
	assert.NoError(t, err)

	latestVersionChain := identityV1Chain

	testcases := []struct {
		name     string
		versions string
		chain    []*x509.Certificate
	}{
		{"V0", "0", identityV0Chain},
		{"V1", "1", identityV1Chain},
		{"any with V0 chain", "*", identityV0Chain},
		{"any with V1 chain", "*", identityV1Chain},
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

func TestIDVersionExtensionHandler_error(t *testing.T) {
	_, identityV1Chain, err := testpeertls.NewCertChain(2, storj.V1)
	assert.NoError(t, err)

	_, identityV2Chain, err := testpeertls.NewCertChain(2, storj.V2)
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

		cert := testcase.chain[peertls.CAIndex]
		ext := cert.Extensions[len(cert.Extensions)-1]
		err := storj.IDVersionHandler.NewHandlerFunc(opts)(ext, identity.ToChains(testcase.chain))
		assert.Error(t, err)

		extensionMap := tlsopts.NewExtensionsMap(testcase.chain...)
		handlerFuncMap := extensions.AllHandlers.WithOptions(opts)

		err = extensionMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.chain))
		assert.Error(t, err)
	}
}

func TestV0LegacyCertCompatibility(t *testing.T) {
	unsigned, err := identity.FullIdentityFromPEM(
		[]byte("-----BEGIN CERTIFICATE-----\nMIIBYTCCAQegAwIBAgIQeCuvxFP1q4uC1Q9LO4SX3TAKBggqhkjOPQQDAjAQMQ4w\nDAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw\nWjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPNe\nsh9wFkAcfE90UPY8VFWJHB7p5Z9CKloXiJdCYcBbv1fiYPHQGJuMV++bO6udhKXI\ngxmfeOn1HNssx+cDXQKjPzA9MA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggr\nBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAKBggqhkjOPQQDAgNIADBF\nAiBuB9ziqPGYHTRUASi/8ssfleI9EL+d0Eh/dafGFZO2HAIhAMe6nZUFrcQiAf3G\ne9ydnkqoISRCZuqIvWDGPkxFjdEp\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBZTCCAQugAwIBAgIQU6pT94OnkHPgQyEK/a1+XzAKBggqhkjOPQQDAjAQMQ4w\nDAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw\nWjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPm1\nBUAtDLSVkjOzV/QHYu68d8/pHPTby0ciGPcAMe6mVcbHql+tKnmjc9ZXAgmGg7wo\nocQ88GP6QI/UOMbLn66jQzBBMA4GA1UdDwEB/wQEAwICBDATBgNVHSUEDDAKBggr\nBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAkGBIg3AgEEAQAwCgYIKoZIzj0EAwID\nSAAwRQIgEJ/xyGghHZOyq/AfTUpgHSocgY2no1XuPTkYQ+zEHJ0CIQCiWx/zp/I5\nTswIOVpPZF0O8SCrHW7L416EK1CSaLCYPA==\n-----END CERTIFICATE-----\n"),
		[]byte("-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEICvE+Bd39LJ3VVf/SBdkw/IPjyVmMWq8Sr7GuWzkfdpJoAoGCCqGSM49\nAwEHoUQDQgAEVoir4M8l+8y/xHAVcikYbEs4cJuy8byIzIBqu/TiXxCm08+dIQqK\nh34oZuFDqkQlnR5YTINuQD1GxQb9rE1f3g==\n-----END EC PRIVATE KEY-----\n"),
	)
	require.NoError(t, err)
	require.Equal(t, unsigned.ID, storj.NodeID{80, 20, 7, 41, 199, 54, 178, 46, 100, 217, 12, 23, 0, 168, 87, 93, 97, 118, 238, 48, 128, 209, 31, 245, 164, 235, 108, 191, 171, 206, 224, 0})

	signed, err := identity.FullIdentityFromPEM(
		[]byte("-----BEGIN CERTIFICATE-----\nMIIBYjCCAQigAwIBAgIRAMM/5SHfNFMLl9uTAAQEoZAwCgYIKoZIzj0EAwIwEDEO\nMAwGA1UEChMFU3RvcmowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw\nMFowEDEOMAwGA1UEChMFU3RvcmowWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAS/\n9wOAe42DV90jcRJMMeGe9os528RNJbMthDMkAn58KyOH87Rvlz0uCRnhhk3AbDE+\nXXHfEyed/HPFEMxJwmlGoz8wPTAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYI\nKwYBBQUHAwEGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwCgYIKoZIzj0EAwIDSAAw\nRQIhALl9VMhM6NFnPblqOsIHOznsKr0OfQREf/+GSk/t8McsAiAxyOYg3IlB9iA0\nq/pD+qUwXuS+NFyVGOhgdNDFT3amOA==\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBWzCCAQGgAwIBAgIRAMfle+YJvbpRwr+FqiTrRyswCgYIKoZIzj0EAwIwEDEO\nMAwGA1UEChMFU3RvcmowIhgPMDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAw\nMFowEDEOMAwGA1UEChMFU3RvcmowWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARL\nO4n2UCp66X/MY5AzhZsfbBYOBw81Dv8V3y1BXXtbHNsUWNY8RT7r5FSTuLHsaXwq\nTwHdU05bjgnLZT/XdwqaozgwNjAOBgNVHQ8BAf8EBAMCAgQwEwYDVR0lBAwwCgYI\nKwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zAKBggqhkjOPQQDAgNIADBFAiEA2vce\nasP0sjt6QRJNkgdV/IONJCF0IGgmsCoogCbh9ggCIA3mHgivRBId7sSAU4UUPxpB\nOOfce7bVuJlxvsnNfkkz\n-----END CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\nMIIBWjCCAQCgAwIBAgIQdzcArqh7Yp9aGiiJXM4+8TAKBggqhkjOPQQDAjAQMQ4w\nDAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw\nWjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABM/W\nTxYhs/yGKSg8+Hb2Z/NB2KJef+fWkq7mHl7vhD9JgFwVMowMEFtKOCAhZxLBZD47\nxhYDhHBv4vrLLS+m3wGjODA2MA4GA1UdDwEB/wQEAwICBDATBgNVHSUEDDAKBggr\nBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49BAMCA0gAMEUCIC+gM/sI\nXXHq5jJmolw50KKVHlqaqpdxjxJ/6x8oqTHWAiEA1w9EbqPXQ5u/oM+ODf1TBkms\nN9NfnJsY1I2A3NKEvq8=\n-----END CERTIFICATE-----\n"),
		[]byte("-----BEGIN PRIVATE KEY-----\nMIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgzsFsVqt/GdqQlIIJ\nHH2VQNndv1A1fTk/35VPNzLW04ehRANCAATzXrIfcBZAHHxPdFD2PFRViRwe6eWf\nQipaF4iXQmHAW79X4mDx0BibjFfvmzurnYSlyIMZn3jp9RzbLMfnA10C\n-----END PRIVATE KEY-----\n"),
	)
	require.NoError(t, err)
	require.Equal(t, signed.ID, storj.NodeID{14, 210, 138, 187, 40, 19, 225, 132, 161, 233, 139, 15, 102, 5, 196, 145, 30, 164, 104, 199, 232, 67, 62, 181, 131, 224, 252, 167, 206, 172, 48, 0})
}
