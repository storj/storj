// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity_test

import (
	"bytes"
	"context"
	"crypto"
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
	"storj.io/storj/internal/testrand"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	require.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	require.NoError(t, err)

	caCert, err := peertls.CreateSelfSignedCertificate(caKey, caTemplate)
	require.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	require.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	require.NoError(t, err)

	leafCert, err := peertls.CreateCertificate(pkcrypto.PublicKeyFromPrivate(leafKey), caKey, leafTemplate, caTemplate)
	require.NoError(t, err)

	peerIdent, err := identity.PeerIdentityFromChain([]*x509.Certificate{leafCert, caCert})
	require.NoError(t, err)
	assert.Equal(t, caCert, peerIdent.CA)
	assert.Equal(t, leafCert, peerIdent.Leaf)
	assert.NotEmpty(t, peerIdent.ID)
}

func TestFullIdentityFromPEM(t *testing.T) {
	caKey, err := pkcrypto.GeneratePrivateKey()
	require.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	require.NoError(t, err)

	caCert, err := peertls.CreateSelfSignedCertificate(caKey, caTemplate)
	require.NoError(t, err)
	require.NoError(t, err)
	require.NotEmpty(t, caCert)

	leafTemplate, err := peertls.LeafTemplate()
	require.NoError(t, err)

	leafKey, err := pkcrypto.GeneratePrivateKey()
	require.NoError(t, err)

	leafCert, err := peertls.CreateCertificate(pkcrypto.PublicKeyFromPrivate(leafKey), caKey, leafTemplate, caTemplate)
	require.NoError(t, err)
	require.NotEmpty(t, leafCert)

	chainPEM := bytes.NewBuffer([]byte{})
	require.NoError(t, pkcrypto.WriteCertPEM(chainPEM, leafCert))
	require.NoError(t, pkcrypto.WriteCertPEM(chainPEM, caCert))

	keyPEM := bytes.NewBuffer([]byte{})
	require.NoError(t, pkcrypto.WritePrivateKeyPEM(keyPEM, leafKey))

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
		identCfg := &identity.Config{
			CertPath: ctx.File("chain.pem"),
			KeyPath:  ctx.File("key.pem"),
		}

		{ // pre-save version assertions
			assert.Equal(t, version.Number, ident.ID.Version().Number)

			caVersion, err := storj.IDVersionFromCert(ident.CA)
			require.NoError(t, err)
			assert.Equal(t, version.Number, caVersion.Number)

			versionExt := tlsopts.NewExtensionsMap(ident.CA)[extensions.IdentityVersionExtID.String()]
			if ident.ID.Version().Number == 0 {
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
			require.NoError(t, err)
			assert.Equal(t, ident.Key, loadedFi.Key)
			assert.Equal(t, ident.Leaf, loadedFi.Leaf)
			assert.Equal(t, ident.CA, loadedFi.CA)
			assert.Equal(t, ident.ID, loadedFi.ID)

			versionExt := tlsopts.NewExtensionsMap(ident.CA)[extensions.IdentityVersionExtID.String()]
			if ident.ID.Version().Number == 0 {
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
		require.NoError(t, pkcrypto.WriteCertPEM(chainPEM, ident.Leaf))
		require.NoError(t, pkcrypto.WriteCertPEM(chainPEM, ident.CA))

		privateKey := ident.Key
		require.NotEmpty(t, privateKey)

		keyPEM := bytes.NewBuffer([]byte{})
		require.NoError(t, pkcrypto.WritePrivateKeyPEM(keyPEM, privateKey))

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

	for _, v := range storj.IDVersions {
		version := v
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
	require.NoError(t, err)
	require.NotNil(t, ca)

	fi, err := ca.NewIdentity()
	require.NoError(t, err)
	require.NotNil(t, fi)

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{fi.Leaf.Raw, fi.CA.Raw}, nil)
	assert.NoError(t, err)
}

func TestManageablePeerIdentity_AddExtension(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	manageablePeerIdentity, err := testidentity.NewTestManageablePeerIdentity(ctx)
	require.NoError(t, err)

	oldLeaf := manageablePeerIdentity.Leaf
	assert.Len(t, manageablePeerIdentity.CA.Cert.ExtraExtensions, 0)

	randBytes := testrand.Bytes(10)
	randExt := pkix.Extension{
		Id:    asn1.ObjectIdentifier{2, 999, int(randBytes[0])},
		Value: randBytes,
	}

	err = manageablePeerIdentity.AddExtension(randExt)
	require.NoError(t, err)

	assert.Len(t, manageablePeerIdentity.Leaf.ExtraExtensions, 0)
	assert.Len(t, manageablePeerIdentity.Leaf.Extensions, len(oldLeaf.Extensions)+1)

	assert.Equal(t, oldLeaf.SerialNumber, manageablePeerIdentity.Leaf.SerialNumber)
	assert.Equal(t, oldLeaf.IsCA, manageablePeerIdentity.Leaf.IsCA)
	assert.Equal(t, oldLeaf.PublicKey, manageablePeerIdentity.Leaf.PublicKey)
	ext := tlsopts.NewExtensionsMap(manageablePeerIdentity.Leaf)[randExt.Id.String()]
	assert.Equal(t, randExt, ext)

	assert.Equal(t, randExt, tlsopts.NewExtensionsMap(manageablePeerIdentity.Leaf)[randExt.Id.String()])

	assert.NotEqual(t, oldLeaf.Raw, manageablePeerIdentity.Leaf.Raw)
	assert.NotEqual(t, oldLeaf.RawTBSCertificate, manageablePeerIdentity.Leaf.RawTBSCertificate)
	assert.NotEqual(t, oldLeaf.Signature, manageablePeerIdentity.Leaf.Signature)
}

func TestManageableFullIdentity_Revoke(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	manageableFullIdentity, err := testidentity.NewTestManageableFullIdentity(ctx)
	require.NoError(t, err)

	oldLeaf := manageableFullIdentity.Leaf
	assert.Len(t, manageableFullIdentity.CA.Cert.ExtraExtensions, 0)

	err = manageableFullIdentity.Revoke()
	require.NoError(t, err)

	assert.Len(t, manageableFullIdentity.Leaf.ExtraExtensions, 0)
	assert.Len(t, manageableFullIdentity.Leaf.Extensions, len(oldLeaf.Extensions)+1)

	assert.Equal(t, oldLeaf.IsCA, manageableFullIdentity.Leaf.IsCA)

	assert.NotEqual(t, oldLeaf.PublicKey, manageableFullIdentity.Leaf.PublicKey)
	assert.NotEqual(t, oldLeaf.SerialNumber, manageableFullIdentity.Leaf.SerialNumber)
	assert.NotEqual(t, oldLeaf.Raw, manageableFullIdentity.Leaf.Raw)
	assert.NotEqual(t, oldLeaf.RawTBSCertificate, manageableFullIdentity.Leaf.RawTBSCertificate)
	assert.NotEqual(t, oldLeaf.Signature, manageableFullIdentity.Leaf.Signature)

	revocationExt := tlsopts.NewExtensionsMap(manageableFullIdentity.Leaf)[extensions.RevocationExtID.String()]
	assert.True(t, extensions.RevocationExtID.Equal(revocationExt.Id))

	var rev extensions.Revocation
	err = rev.Unmarshal(revocationExt.Value)
	require.NoError(t, err)

	err = rev.Verify(manageableFullIdentity.CA.Cert)
	require.NoError(t, err)
}
