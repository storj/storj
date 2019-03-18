// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity_test

import (
	"context"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/storj"
)

func TestNewCA(t *testing.T) {
	const expectedDifficulty = 4

	for _, version := range storj.IDVersions {
		ca, err := identity.NewCA(context.Background(), identity.NewCAOptions{
			Version:     version,
			Difficulty:  expectedDifficulty,
			Concurrency: 4,
		})
		assert.NoError(t, err)
		require.NotEmpty(t, ca)

		assert.Equal(t, version.Number, ca.ID.Version().Number)

		actualDifficulty, err := ca.ID.Difficulty()
		assert.NoError(t, err)
		assert.True(t, actualDifficulty >= expectedDifficulty)
	}
}

func TestFullCertificateAuthority_NewIdentity(t *testing.T) {
	ctx := testcontext.New(t)
	ca, err := identity.NewCA(ctx, identity.NewCAOptions{
		Difficulty:  12,
		Concurrency: 4,
	})
	if !assert.NoError(t, err) || !assert.NotNil(t, ca) {
		t.Fatal(err)
	}

	fi, err := ca.NewIdentity()
	if !assert.NoError(t, err) || !assert.NotNil(t, fi) {
		t.Fatal(err)
	}

	assert.Equal(t, ca.Cert, fi.CA)
	assert.Equal(t, ca.ID, fi.ID)
	assert.NotEqual(t, ca.Key, fi.Key)
	assert.NotEqual(t, ca.Cert, fi.Leaf)

	err = fi.Leaf.CheckSignatureFrom(ca.Cert)
	assert.NoError(t, err)
}

func TestFullCertificateAuthority_Sign(t *testing.T) {
	ctx := testcontext.New(t)
	caOpts := identity.NewCAOptions{
		Difficulty:  12,
		Concurrency: 4,
	}

	ca, err := identity.NewCA(ctx, caOpts)
	if !assert.NoError(t, err) || !assert.NotNil(t, ca) {
		t.Fatal(err)
	}

	toSign, err := identity.NewCA(ctx, caOpts)
	if !assert.NoError(t, err) || !assert.NotNil(t, toSign) {
		t.Fatal(err)
	}

	signed, err := ca.Sign(toSign.Cert)
	if !assert.NoError(t, err) || !assert.NotNil(t, signed) {
		t.Fatal(err)
	}

	assert.Equal(t, toSign.Cert.RawTBSCertificate, signed.RawTBSCertificate)
	assert.NotEqual(t, toSign.Cert.Signature, signed.Signature)
	assert.NotEqual(t, toSign.Cert.Raw, signed.Raw)

	err = signed.CheckSignatureFrom(ca.Cert)
	assert.NoError(t, err)
}

func TestFullCAConfig_Save(t *testing.T) {
	// TODO(bryanchriswhite): test with both
	// TODO(bryanchriswhite): test with only cert path
	// TODO(bryanchriswhite): test with only key path
	t.SkipNow()
}

func TestFullCAConfig_Load_extensions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	for _, version := range storj.IDVersions {
		caCfg := identity.CASetupConfig{
			CertPath: ctx.File("ca.cert"),
			KeyPath:  ctx.File("ca.key"),
		}

		{
			ca, err := caCfg.Create(ctx, version, nil)
			require.NoError(t, err)

			caVersion, err := ca.Version()
			require.NoError(t, err)
			require.Equal(t, version.Number, caVersion.Number)
		}

		{
			ca, err := caCfg.FullConfig().Load()
			require.NoError(t, err)
			caVersion, err := ca.Version()
			require.NoError(t, err)
			assert.Equal(t, version.Number, caVersion.Number)
		}

	}
}

func BenchmarkNewCA(b *testing.B) {
	ctx := context.Background()
	for _, difficulty := range []uint16{8, 12} {
		for _, concurrency := range []uint{1, 2, 5, 10} {
			test := fmt.Sprintf("%d/%d", difficulty, concurrency)
			b.Run(test, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					_, _ = identity.NewCA(ctx, identity.NewCAOptions{
						Difficulty:  difficulty,
						Concurrency: concurrency,
					})
				}
			})
		}
	}
}

func TestFullCertificateAuthority_AddExtension(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ca, err := testidentity.NewTestCA(ctx)
	require.NoError(t, err)

	oldCert := ca.Cert
	assert.Len(t, ca.Cert.ExtraExtensions, 0)

	randBytes := make([]byte, 10)
	rand.Read(randBytes)
	randExt := pkix.Extension{
		Id:    asn1.ObjectIdentifier{2, 999, int(randBytes[0])},
		Value: randBytes,
	}

	err = ca.AddExtension(randExt)
	assert.NoError(t, err)

	assert.Len(t, ca.Cert.ExtraExtensions, 0)
	assert.Len(t, ca.Cert.Extensions, len(oldCert.Extensions)+1)

	assert.Equal(t, oldCert.SerialNumber, ca.Cert.SerialNumber)
	assert.Equal(t, oldCert.IsCA, ca.Cert.IsCA)
	assert.Equal(t, oldCert.PublicKey, ca.Cert.PublicKey)
	assert.Equal(t, randExt, ca.Cert.Extensions[len(ca.Cert.Extensions)-1])

	assert.NotEqual(t, oldCert.Raw, ca.Cert.Raw)
	assert.NotEqual(t, oldCert.RawTBSCertificate, ca.Cert.RawTBSCertificate)
	assert.NotEqual(t, oldCert.Signature, ca.Cert.Signature)
}

func TestFullCertificateAuthority_Revoke(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ca, err := testidentity.NewTestCA(ctx)
	require.NoError(t, err)

	oldCert := ca.Cert
	assert.Len(t, ca.Cert.ExtraExtensions, 0)

	err = ca.Revoke()
	assert.NoError(t, err)

	assert.Len(t, ca.Cert.ExtraExtensions, 0)
	assert.Len(t, ca.Cert.Extensions, len(oldCert.Extensions)+1)

	assert.Equal(t, oldCert.SerialNumber, ca.Cert.SerialNumber)
	assert.Equal(t, oldCert.IsCA, ca.Cert.IsCA)
	assert.Equal(t, oldCert.PublicKey, ca.Cert.PublicKey)

	assert.NotEqual(t, oldCert.Raw, ca.Cert.Raw)
	assert.NotEqual(t, oldCert.RawTBSCertificate, ca.Cert.RawTBSCertificate)
	assert.NotEqual(t, oldCert.Signature, ca.Cert.Signature)

	revocationExt := ca.Cert.Extensions[len(ca.Cert.Extensions)-1]
	assert.True(t, extensions.RevocationExtID.Equal(revocationExt.Id))

	var rev extensions.Revocation
	err = rev.Unmarshal(revocationExt.Value)
	require.NoError(t, err)

	err = rev.Verify(ca.Cert)
	assert.NoError(t, err)
}
