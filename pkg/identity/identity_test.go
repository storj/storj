// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509/pkix"
	"encoding/asn1"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, caKey, caTemplate, nil)
	assert.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)

	peerIdent, err := identity.PeerIdentityFromCerts(leafCert, caCert, nil)
	assert.NoError(t, err)
	assert.Equal(t, caCert, peerIdent.CA)
	assert.Equal(t, leafCert, peerIdent.Leaf)
	assert.NotEmpty(t, peerIdent.ID)
}

func TestFullIdentityFromPEM(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, caKey, caTemplate, nil)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.NotEmpty(t, caCert)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)
	assert.NotEmpty(t, leafCert)

	chainPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pkcrypto.WriteCertPEM(chainPEM, leafCert))
	assert.NoError(t, pkcrypto.WriteCertPEM(chainPEM, caCert))

	keyPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pkcrypto.WritePrivateKeyPEM(keyPEM, leafKey))

	fullIdent, err := identity.FullIdentityFromPEM(chainPEM.Bytes(), keyPEM.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, leafCert.Raw, fullIdent.Leaf.Raw)
	assert.Equal(t, caCert.Raw, fullIdent.CA.Raw)
	assert.Equal(t, leafKey, fullIdent.Key)
}

func TestConfig_SaveIdentity(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ic := &identity.Config{
		CertPath: ctx.File("chain.pem"),
		KeyPath:  ctx.File("key.pem"),
	}
	fi := pregeneratedIdentity(t)

	chainPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pkcrypto.WriteCertPEM(chainPEM, fi.Leaf))
	assert.NoError(t, pkcrypto.WriteCertPEM(chainPEM, fi.CA))

	privateKey := fi.Key
	assert.NotEmpty(t, privateKey)

	keyPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pkcrypto.WritePrivateKeyPEM(keyPEM, privateKey))

	{ // test saving
		err := ic.Save(fi)
		assert.NoError(t, err)

		certInfo, err := os.Stat(ic.CertPath)
		assert.NoError(t, err)

		keyInfo, err := os.Stat(ic.KeyPath)
		assert.NoError(t, err)

		// TODO (windows): ignoring for windows due to different default permissions
		if runtime.GOOS != "windows" {
			assert.Equal(t, os.FileMode(0644), certInfo.Mode())
			assert.Equal(t, os.FileMode(0600), keyInfo.Mode())
		}
	}

	{ // test loading
		loadedFi, err := ic.Load()
		assert.NoError(t, err)
		assert.Equal(t, fi.Key, loadedFi.Key)
		assert.Equal(t, fi.Leaf, loadedFi.Leaf)
		assert.Equal(t, fi.CA, loadedFi.CA)
		assert.Equal(t, fi.ID, loadedFi.ID)
	}
}

func TestVerifyPeer(t *testing.T) {
	ca, err := identity.NewCA(context.Background(), identity.NewCAOptions{
		Difficulty:  12,
		Concurrency: 4,
	})
	assert.NoError(t, err)

	fi, err := ca.NewIdentity()
	assert.NoError(t, err)

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{fi.Leaf.Raw, fi.CA.Raw}, nil)
	assert.NoError(t, err)
}

func TestManageableIdentity_AddExtension(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ca, err := testidentity.NewTestCA(ctx)
	require.NoError(t, err)

	ident, err := ca.NewIdentity()
	manIdent := identity.NewManageableIdentity(ident, ca)

	oldLeaf := manIdent.Leaf
	assert.Len(t, ca.Cert.ExtraExtensions, 0)

	randBytes := make([]byte, 10)
	rand.Read(randBytes)
	randExt := pkix.Extension{
		Id:    asn1.ObjectIdentifier{2, 999, int(randBytes[0])},
		Value: randBytes,
	}

	err = manIdent.AddExtension(randExt)
	assert.NoError(t, err)

	assert.Len(t, manIdent.Leaf.ExtraExtensions, 0)
	assert.Len(t, manIdent.Leaf.Extensions, len(oldLeaf.Extensions)+1)

	assert.Equal(t, oldLeaf.SerialNumber, manIdent.Leaf.SerialNumber)
	assert.Equal(t, oldLeaf.IsCA, manIdent.Leaf.IsCA)
	assert.Equal(t, oldLeaf.PublicKey, manIdent.Leaf.PublicKey)
	assert.Equal(t, randExt, manIdent.Leaf.Extensions[len(manIdent.Leaf.Extensions)-1])

	assert.NotEqual(t, oldLeaf.Raw, manIdent.Leaf.Raw)
	assert.NotEqual(t, oldLeaf.RawTBSCertificate, manIdent.Leaf.RawTBSCertificate)
	assert.NotEqual(t, oldLeaf.Signature, manIdent.Leaf.Signature)
}

func TestManageableIdentity_Revoke(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ca, err := testidentity.NewTestCA(ctx)
	require.NoError(t, err)

	ident, err := ca.NewIdentity()
	manIdent := identity.NewManageableIdentity(ident, ca)

	oldLeaf := manIdent.Leaf
	assert.Len(t, ca.Cert.ExtraExtensions, 0)

	err = manIdent.Revoke()
	assert.NoError(t, err)

	assert.Len(t, manIdent.Leaf.ExtraExtensions, 0)
	assert.Len(t, manIdent.Leaf.Extensions, len(oldLeaf.Extensions)+1)

	assert.Equal(t, oldLeaf.IsCA, manIdent.Leaf.IsCA)
	assert.Equal(t, oldLeaf.PublicKey, manIdent.Leaf.PublicKey)

	assert.NotEqual(t, oldLeaf.SerialNumber, manIdent.Leaf.SerialNumber)
	assert.NotEqual(t, oldLeaf.Raw, manIdent.Leaf.Raw)
	assert.NotEqual(t, oldLeaf.RawTBSCertificate, manIdent.Leaf.RawTBSCertificate)
	assert.NotEqual(t, oldLeaf.Signature, manIdent.Leaf.Signature)

	revocationExt := manIdent.Leaf.Extensions[len(manIdent.Leaf.Extensions)-1]
	assert.True(t, extensions.RevocationExtID.Equal(revocationExt.Id))

	var rev extensions.Revocation
	err = rev.Unmarshal(revocationExt.Value)
	require.NoError(t, err)

	err = rev.Verify(ca.Cert)
	assert.NoError(t, err)
}

func pregeneratedIdentity(t *testing.T) *identity.FullIdentity {
	const chain = `-----BEGIN CERTIFICATE-----
MIIBQDCB56ADAgECAhB+u3d03qyW/ROgwy/ZsPccMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIZrEPV/ExEkF0qUF0fJ3qSeGt5oFUX231v02NSUywcQ/Ve0
v3nHbmcJdjWBis2AkfL25mYDVC25jLl4tylMKumjPzA9MA4GA1UdDwEB/wQEAwIF
oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
BggqhkjOPQQDAgNIADBFAiEA2ZvsR0ncw4mHRIg2Isavd+XVEoMo/etXQRAkDy9n
wyoCIDykUsqjshc9kCrXOvPSN8GuO2bNoLu5C7K1GlE/HI2X
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBODCB4KADAgECAhAOcvhKe5TWT44LqFfgA1f8MAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIZrEPV/ExEkF0qUF0fJ3qSeGt5oFUX231v02NSUywcQ/Ve0
v3nHbmcJdjWBis2AkfL25mYDVC25jLl4tylMKumjODA2MA4GA1UdDwEB/wQEAwIC
BDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49
BAMCA0cAMEQCIGAZfPT1qvlnkTacojTtP20ZWf6XbnSztJHIKlUw6AE+AiB5Vcjj
awRaC5l1KBPGqiKB0coVXDwhW+K70l326MPUcg==
-----END CERTIFICATE-----`

	const key = `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIKGjEetrxKrzl+AL1E5LXke+1ElyAdjAmr88/1Kx09+doAoGCCqGSM49
AwEHoUQDQgAEoLy/0hs5deTXZunRumsMkiHpF0g8wAc58aXANmr7Mxx9tzoIYFnx
0YN4VDKdCtUJa29yA6TIz1MiIDUAcB5YCA==
-----END EC PRIVATE KEY-----`

	fi, err := identity.FullIdentityFromPEM([]byte(chain), []byte(key))
	assert.NoError(t, err)

	return fi
}
