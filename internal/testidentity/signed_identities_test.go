package testidentity

import (
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/storj"
)

type void struct{}

func TestPregeneratedIdentity(t *testing.T) {
	seenIDVersions := make(map[storj.IDVersionNumber]void)
	IdentityVersionsTest(t, func(t *testing.T, ident *identity.FullIdentity) {
		seenIDVersions[ident.ID.Version().Number] = void{}
		fmt.Printf("version %d\n", ident.ID.Version().Number)

		chains := [][]*x509.Certificate{
			append([]*x509.Certificate{ident.Leaf, ident.CA}, ident.RestChain...),
		}

		err := peertls.VerifyPeerCertChains(nil, chains)
		assert.NoError(t, err)
	})

	assert.Len(t, seenIDVersions, len(storj.IDVersions))
}

func TestPregeneratedSignedIdentity(t *testing.T) {
	seenIDVersions := make(map[storj.IDVersionNumber]void)
	fmt.Printf("pregenerated versions %+v\n", IdentityVersions)
	SignedIdentityVersionsTest(t, func(t *testing.T, ident *identity.FullIdentity) {
		seenIDVersions[ident.ID.Version().Number] = void{}
		fmt.Printf("version %d\n", ident.ID.Version().Number)

		chains := [][]*x509.Certificate{
			append([]*x509.Certificate{ident.Leaf, ident.CA}, ident.RestChain...),
		}

		err := peertls.VerifyPeerCertChains(nil, chains)
		assert.NoError(t, err)

		signer := NewPregeneratedSigner(ident.ID.Version())
		err = peertls.VerifyCAWhitelist([]*x509.Certificate{signer.Cert})(nil, chains)
		assert.NoError(t, err)
	})

	assert.Len(t, seenIDVersions, len(storj.IDVersions))
}
