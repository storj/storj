// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity_test

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewSelfSignedCert(caKey, caTemplate)
	assert.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(pkcrypto.PublicKeyFromPrivate(leafKey), caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)

	peerIdent, err := identity.PeerIdentityFromChain([]*x509.Certificate{leafCert, caCert})
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

	caCert, err := peertls.NewSelfSignedCert(caKey, caTemplate)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.NotEmpty(t, caCert)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(pkcrypto.PublicKeyFromPrivate(leafKey), caKey, leafTemplate, caTemplate)
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

func TestConfig_Save_with_extension(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, ident *identity.FullIdentity) {
		// TODO: fix extension serialization
		if version.Number == storj.V2 {
			t.Skipf("certificate extension serialization fix required")
		}

		identCfg := &identity.Config{
			CertPath: ctx.File("chain.pem"),
			KeyPath:  ctx.File("key.pem"),
		}

		{ // pre-save version assertions
			assert.Equal(t, version.Number, ident.ID.Version().Number)

			caVersion, err := storj.IDVersionFromCert(ident.CA)
			require.NoError(t, err)
			assert.Equal(t, version.Number, caVersion.Number)

			var versionExt pkix.Extension
			for _, ext := range ident.CA.ExtraExtensions {
				if ext.Id.Equal(extensions.IdentityVersionExtID) {
					versionExt = ext
					break
				}
			}

			if ident.ID.Version().Number == 1 {
				require.NotEmpty(t, versionExt)
				assert.Equal(t, ident.ID.Version().Number, storj.IDVersionNumber(versionExt.Value[0]))
			} else {
				assert.Empty(t, versionExt)
			}
		}

		{ // test saving
			err := identCfg.Save(ident)
			assert.NoError(t, err)

			certInfo, err := os.Stat(identCfg.CertPath)
			assert.NoError(t, err)

			keyInfo, err := os.Stat(identCfg.KeyPath)
			assert.NoError(t, err)

			// TODO (windows): ignoring for windows due to different default permissions
			if runtime.GOOS != "windows" {
				assert.Equal(t, os.FileMode(0644), certInfo.Mode())
				assert.Equal(t, os.FileMode(0600), keyInfo.Mode())
			}
		}

		{ // test loading
			loadedFi, err := identCfg.Load()
			assert.NoError(t, err)
			assert.Equal(t, ident.Key, loadedFi.Key)
			assert.Equal(t, ident.Leaf, loadedFi.Leaf)
			assert.Equal(t, ident.CA, loadedFi.CA)
			assert.Equal(t, ident.ID, loadedFi.ID)

			var versionExt pkix.Extension
			for _, ext := range ident.CA.ExtraExtensions {
				if ext.Id.Equal(extensions.IdentityVersionExtID) {
					versionExt = ext
					break
				}
			}

			if ident.ID.Version().Number == 1 {
				require.NotEmpty(t, versionExt)
				assert.Equal(t, ident.ID.Version().Number, storj.IDVersionNumber(versionExt.Value[0]))
			} else {
				assert.Empty(t, versionExt)
			}
		}
	})
}

func TestConfig_Save(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.IdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, ident *identity.FullIdentity) {
		identCfg := &identity.Config{
			CertPath: ctx.File("chain.pem"),
			KeyPath:  ctx.File("key.pem"),
		}

		chainPEM := bytes.NewBuffer([]byte{})
		assert.NoError(t, pkcrypto.WriteCertPEM(chainPEM, ident.Leaf))
		assert.NoError(t, pkcrypto.WriteCertPEM(chainPEM, ident.CA))

		privateKey := ident.Key
		assert.NotEmpty(t, privateKey)

		keyPEM := bytes.NewBuffer([]byte{})
		assert.NoError(t, pkcrypto.WritePrivateKeyPEM(keyPEM, privateKey))

		{ // test saving
			err := identCfg.Save(ident)
			assert.NoError(t, err)

			certInfo, err := os.Stat(identCfg.CertPath)
			assert.NoError(t, err)

			keyInfo, err := os.Stat(identCfg.KeyPath)
			assert.NoError(t, err)

			// TODO (windows): ignoring for windows due to different default permissions
			if runtime.GOOS != "windows" {
				assert.Equal(t, os.FileMode(0644), certInfo.Mode())
				assert.Equal(t, os.FileMode(0600), keyInfo.Mode())
			}
		}

		{ // test loading
			loadedFi, err := identCfg.Load()
			assert.NoError(t, err)
			assert.Equal(t, ident.Key, loadedFi.Key)
			assert.Equal(t, ident.Leaf, loadedFi.Leaf)
			assert.Equal(t, ident.CA, loadedFi.CA)
			assert.Equal(t, ident.ID, loadedFi.ID)
		}
	})
}

func TestVersionedNodeIDFromKey(t *testing.T) {
	_, chain, err := testpeertls.NewCertChain(1, storj.LatestIDVersion().Number)
	require.NoError(t, err)

	pubKey, ok := chain[peertls.LeafIndex].PublicKey.(crypto.PublicKey)
	require.True(t, ok)

	for _, version := range storj.IDVersions {
		t.Run(fmt.Sprintf("IdentityV%d", version.Number), func(t *testing.T) {
			id, err := identity.NodeIDFromKey(pubKey, version)
			require.NoError(t, err)
			assert.Equal(t, version.Number, id.Version().Number)
		})
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

	manageableIdentity, err := testidentity.NewTestManageablePeerIdentity(ctx)
	require.NoError(t, err)

	oldLeaf := manageableIdentity.Leaf
	assert.Len(t, manageableIdentity.CA.Cert.ExtraExtensions, 0)

	randBytes := make([]byte, 10)
	_, err = rand.Read(randBytes)
	require.NoError(t, err)
	randExt := pkix.Extension{
		Id:    asn1.ObjectIdentifier{2, 999, int(randBytes[0])},
		Value: randBytes,
	}

	err = manageableIdentity.AddExtension(randExt)
	assert.NoError(t, err)

	assert.Len(t, manageableIdentity.Leaf.ExtraExtensions, 0)
	assert.Len(t, manageableIdentity.Leaf.Extensions, len(oldLeaf.Extensions)+1)

	assert.Equal(t, oldLeaf.SerialNumber, manageableIdentity.Leaf.SerialNumber)
	assert.Equal(t, oldLeaf.IsCA, manageableIdentity.Leaf.IsCA)
	assert.Equal(t, oldLeaf.PublicKey, manageableIdentity.Leaf.PublicKey)
	assert.Equal(t, randExt, manageableIdentity.Leaf.Extensions[len(manageableIdentity.Leaf.Extensions)-1])

	assert.NotEqual(t, oldLeaf.Raw, manageableIdentity.Leaf.Raw)
	assert.NotEqual(t, oldLeaf.RawTBSCertificate, manageableIdentity.Leaf.RawTBSCertificate)
	assert.NotEqual(t, oldLeaf.Signature, manageableIdentity.Leaf.Signature)
}

func TestManageableIdentity_Revoke(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	manageableFullIdent, err := testidentity.NewTestManageableFullIdentity(ctx)
	require.NoError(t, err)

	oldLeaf := manageableFullIdent.Leaf
	assert.Len(t, manageableFullIdent.CA.Cert.ExtraExtensions, 0)

	err = manageableFullIdent.Revoke()
	assert.NoError(t, err)

	assert.Len(t, manageableFullIdent.Leaf.ExtraExtensions, 0)
	assert.Len(t, manageableFullIdent.Leaf.Extensions, len(oldLeaf.Extensions)+1)

	assert.Equal(t, oldLeaf.IsCA, manageableFullIdent.Leaf.IsCA)

	assert.NotEqual(t, oldLeaf.PublicKey, manageableFullIdent.Leaf.PublicKey)
	assert.NotEqual(t, oldLeaf.SerialNumber, manageableFullIdent.Leaf.SerialNumber)
	assert.NotEqual(t, oldLeaf.Raw, manageableFullIdent.Leaf.Raw)
	assert.NotEqual(t, oldLeaf.RawTBSCertificate, manageableFullIdent.Leaf.RawTBSCertificate)
	assert.NotEqual(t, oldLeaf.Signature, manageableFullIdent.Leaf.Signature)

	revocationExt := manageableFullIdent.Leaf.Extensions[len(manageableFullIdent.Leaf.Extensions)-1]
	assert.True(t, extensions.RevocationExtID.Equal(revocationExt.Id))

	var rev extensions.Revocation
	err = rev.Unmarshal(revocationExt.Value)
	require.NoError(t, err)

	err = rev.Verify(manageableFullIdent.CA.Cert)
	assert.NoError(t, err)
}
