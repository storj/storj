package testplanet

import (
	"crypto/x509"
	"storj.io/storj/pkg/storj"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/peertls"
)

func TestPregeneratedIdentity(t *testing.T) {
	ident, err := PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	chains := [][]*x509.Certificate{
		append([]*x509.Certificate{ident.Leaf, ident.CA}, ident.RestChain...),
	}

	err = peertls.VerifyPeerCertChains(nil, chains)
	assert.NoError(t, err)
}

func TestPregeneratedSignedIdentity(t *testing.T) {
	ident, err := PregeneratedSignedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	chains := [][]*x509.Certificate{
		append([]*x509.Certificate{ident.Leaf, ident.CA}, ident.RestChain...),
	}

	err = peertls.VerifyPeerCertChains(nil, chains)
	assert.NoError(t, err)

	signer := NewPregeneratedSigner(storj.IDVersions[storj.LatestIDVersion().Number])
	err = peertls.VerifyCAWhitelist([]*x509.Certificate{signer.Cert})(nil, chains)
	assert.NoError(t, err)
}
