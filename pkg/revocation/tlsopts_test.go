// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package revocation_test

import (
	"crypto/x509"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/peertls"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/testpeertls"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/private/testrevocation"
	"storj.io/storj/storage"
)

func TestNewOptions(t *testing.T) {
	// TODO: this is not a great test...
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fi, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
	require.NoError(t, err)

	whitelistPath := ctx.File("whitelist.pem")

	chainData, err := peertls.ChainBytes(fi.CA)
	assert.NoError(t, err)

	err = ioutil.WriteFile(whitelistPath, chainData, 0644)
	assert.NoError(t, err)

	cases := []struct {
		testID                     string
		config                     tlsopts.Config
		clientVerificationFuncsLen int
		serverVerificationFuncsLen int
	}{
		{
			"default",
			tlsopts.Config{},
			1, 1,
		}, {
			"revocation processing",
			tlsopts.Config{
				RevocationDBURL: "bolt://" + ctx.File("revocation1.db"),
				Extensions: extensions.Config{
					Revocation: true,
				},
			},
			1, 1,
		}, {
			"ca whitelist verification",
			tlsopts.Config{
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
			},
			2, 1,
		}, {
			"ca whitelist verification and whitelist signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				Extensions: extensions.Config{
					WhitelistSignedLeaf: true,
				},
			},
			2, 1,
		}, {
			"revocation processing and whitelist verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation2.db"),
				Extensions: extensions.Config{
					Revocation: true,
				},
			},
			2, 1,
		}, {
			"revocation processing, whitelist, and signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation3.db"),
				Extensions: extensions.Config{
					Revocation:          true,
					WhitelistSignedLeaf: true,
				},
			},
			2, 1,
		},
	}

	for _, c := range cases {
		t.Log(c.testID)

		revocationDB, err := revocation.NewDBFromCfg(c.config)
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(fi, c.config, revocationDB)
		assert.NoError(t, err)
		assert.True(t, reflect.DeepEqual(fi, tlsOptions.Ident))
		assert.Equal(t, c.config, tlsOptions.Config)
		assert.Len(t, tlsOptions.VerificationFuncs.Client(), c.clientVerificationFuncsLen)
		assert.Len(t, tlsOptions.VerificationFuncs.Server(), c.serverVerificationFuncsLen)

		require.NoError(t, revocationDB.Close())
	}
}
func TestExtensionMap_HandleExtensions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testidentity.IdentityVersionsTest(t, func(t *testing.T, version storj.IDVersion, _ *identity.FullIdentity) {
		keys, originalChain, err := testpeertls.NewCertChain(2, version.Number)
		assert.NoError(t, err)

		rev := new(extensions.Revocation)

		oldRevokedLeafChain, revocationExt, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], originalChain)
		require.NoError(t, err)
		err = rev.Unmarshal(revocationExt.Value)
		require.NoError(t, err)
		err = rev.Verify(oldRevokedLeafChain[peertls.CAIndex])
		require.NoError(t, err)

		// NB: node ID is the same, timestamp must change
		// (see: identity.RevocationDB#Put)
		time.Sleep(1 * time.Second)
		newRevokedLeafChain, revocationExt, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], oldRevokedLeafChain)
		require.NoError(t, err)
		err = rev.Unmarshal(revocationExt.Value)
		require.NoError(t, err)
		err = rev.Verify(newRevokedLeafChain[peertls.CAIndex])
		require.NoError(t, err)

		testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
			opts := &extensions.Options{
				RevocationDB:   revDB,
				PeerIDVersions: "*",
			}

			testcases := []struct {
				name  string
				chain []*x509.Certificate
			}{
				{"no extensions", originalChain},
				{"leaf revocation", oldRevokedLeafChain},
				{"double leaf revocation", newRevokedLeafChain},
				// TODO: more and more diverse extensions in cases
			}

			{
				handlerFuncMap := extensions.DefaultHandlers.WithOptions(opts)
				for _, testcase := range testcases {
					t.Log(testcase.name)
					extensionsMap := tlsopts.NewExtensionsMap(testcase.chain...)
					err := extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.chain))
					assert.NoError(t, err)
				}
			}
		})
	})
}

func TestExtensionMap_HandleExtensions_error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testrevocation.RunDBs(t, func(t *testing.T, revDB extensions.RevocationDB, db storage.KeyValueStore) {
		keys, chain, oldRevocation, err := testpeertls.NewRevokedLeafChain()
		assert.NoError(t, err)

		// NB: node ID is the same, timestamp must change
		// (see: identity.RevocationDB#Put)
		time.Sleep(time.Second)
		_, newRevocation, err := testpeertls.RevokeLeaf(keys[peertls.CAIndex], chain)
		require.NoError(t, err)

		assert.NotEqual(t, oldRevocation, newRevocation)

		err = revDB.Put(ctx, chain, newRevocation)
		assert.NoError(t, err)

		opts := &extensions.Options{RevocationDB: revDB}
		handlerFuncMap := extensions.HandlerFactories{
			extensions.RevocationUpdateHandler,
		}.WithOptions(opts)
		extensionsMap := tlsopts.NewExtensionsMap(chain[peertls.LeafIndex])

		assert.Equal(t, oldRevocation, extensionsMap[extensions.RevocationExtID.String()])

		err = extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(chain))
		assert.Errorf(t, err, extensions.ErrRevocationTimestamp.Error())
	})
}
