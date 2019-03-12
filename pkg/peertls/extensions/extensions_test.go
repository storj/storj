// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
)

func TestParseExtensions(t *testing.T) {
	// TODO: separate this into multiple tests!
	// TODO: this is not a great test
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	revokedLeafKeys, revokedLeafChain, _, err := testpeertls.NewRevokedLeafChain()
	assert.NoError(t, err)

	whitelistSignedKeys, whitelistSignedChain, err := testpeertls.NewCertChain(3)
	assert.NoError(t, err)

	err = extensions.AddSignedCertExt(whitelistSignedKeys[0], whitelistSignedChain[0])
	assert.NoError(t, err)

	_, unrelatedChain, err := testpeertls.NewCertChain(1)
	assert.NoError(t, err)

	revDB, err := identity.NewRevocationDBBolt(ctx.File("revocations.db"))
	assert.NoError(t, err)
	defer ctx.Check(revDB.Close)

	testcases := []struct {
		name      string
		config    extensions.Config
		certChain []*x509.Certificate
		whitelist []*x509.Certificate
		errClass  *errs.Class
		err       error
	}{
		{
			"leaf whitelist signature - success",
			extensions.Config{WhitelistSignedLeaf: true},
			whitelistSignedChain,
			[]*x509.Certificate{whitelistSignedChain[2]},
			nil,
			nil,
		},
		{
			"leaf whitelist signature - failure (empty whitelist)",
			extensions.Config{WhitelistSignedLeaf: true},
			whitelistSignedChain,
			nil,
			&extensions.Error,
			nil,
		},
		{
			"leaf whitelist signature - failure",
			extensions.Config{WhitelistSignedLeaf: true},
			whitelistSignedChain,
			unrelatedChain,
			&extensions.Error,
			nil,
		},
		{
			"certificate revocation - single revocation ",
			extensions.Config{Revocation: true},
			revokedLeafChain,
			nil,
			nil,
			nil,
		},
		{
			"certificate revocation - serial revocations",
			extensions.Config{Revocation: true},
			func() []*x509.Certificate {
				rev := new(extensions.Revocation)
				time.Sleep(1 * time.Second)
				chain, revocationExt, err := testpeertls.RevokeLeaf(revokedLeafKeys, revokedLeafChain)
				assert.NoError(t, err)

				err = rev.Unmarshal(revocationExt.Value)
				assert.NoError(t, err)

				return chain
			}(),
			nil,
			nil,
			nil,
		},
		{
			"certificate revocation - serial revocations error (older timestamp)",
			extensions.Config{Revocation: true},
			func() []*x509.Certificate {
				keys, chain, _, err := testpeertls.NewRevokedLeafChain()
				assert.NoError(t, err)

				rev := new(extensions.Revocation)
				err = rev.Unmarshal(chain[0].ExtraExtensions[0].Value)
				assert.NoError(t, err)

				rev.Timestamp = rev.Timestamp + 300
				err = rev.Sign(keys[0])
				assert.NoError(t, err)

				revBytes, err := rev.Marshal()
				assert.NoError(t, err)

				err = revDB.Put(chain, pkix.Extension{
					Id:    extensions.RevocationExtID.ToASN1(),
					Value: revBytes,
				})
				assert.NoError(t, err)
				return chain
			}(),
			nil,
			&extensions.Error,
			extensions.ErrRevocationTimestamp,
		},
		{
			"certificate revocation and leaf whitelist signature",
			extensions.Config{Revocation: true, WhitelistSignedLeaf: true},
			func() []*x509.Certificate {
				_, chain, _, err := testpeertls.NewRevokedLeafChain()
				assert.NoError(t, err)

				err = extensions.AddSignedCertExt(whitelistSignedKeys[0], chain[0])
				assert.NoError(t, err)

				return chain
			}(),
			[]*x509.Certificate{whitelistSignedChain[2]},
			nil,
			nil,
		},
	}

	for _, testcase := range testcases {
		t.Run(testcase.name, func(t *testing.T) {
			opts := &extensions.Options{
				PeerCAWhitelist: testcase.whitelist,
				RevDB:           revDB,
			}

			handlerFuncMap := extensions.AllHandlers.WithOptions(opts)
			extensionsMap := tlsopts.NewExtensionsMap(testcase.certChain...)
			err := extensionsMap.HandleExtensions(handlerFuncMap, identity.ToChains(testcase.certChain))
			if testcase.errClass != nil {
				assert.True(t, testcase.errClass.Has(err))
			}
			if testcase.err != nil {
				assert.NotNil(t, err)
			}
			if testcase.errClass == nil && testcase.err == nil {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHandlers_Register(t *testing.T) {
	var (
		handlers extensions.HandlerFactories
		ids      []*extensions.ExtensionID
		opts     []*extensions.Options
		exts     []pkix.Extension
		chains   [][][]*x509.Certificate
	)
	limit := 5

	iterate := func(limit int, f func(int)) {
		for i := 0; i < 5; i++ {
			f(i)
		}
	}

	iterate(limit, func(i int) {
		ids = append(ids, &extensions.ExtensionID{2, 999, 999, i})
		opts = append(opts, &extensions.Options{})
		exts = append(exts, pkix.Extension{Id: ids[i].ToASN1()})

		_, chain, err := testpeertls.NewCertChain(2)
		require.NoError(t, err)
		chains = append(chains, identity.ToChains(chain))
	})

	iterate(limit, func(i int) {
		testHandler := extensions.NewHandler(
			ids[i],
			func(opt *extensions.Options) extensions.HandlerFunc {
				assert.Equal(t, opts[i], opt)
				assert.NotNil(t, opt)

				return func(ext pkix.Extension, chain [][]*x509.Certificate) error {
					assert.NotNil(t, ext)
					assert.Equal(t, exts[i], ext)

					assert.NotNil(t, ext.Id)
					assert.Equal(t, ids[i].ToASN1(), ext.Id)

					assert.NotNil(t, chain)
					assert.Equal(t, chains[i], chain)
					return errs.New(strconv.Itoa(i))
				}
			},
		)
		handlers.Register(testHandler)
	})

	iterate(limit, func(i int) {
		err := handlers[i].NewHandlerFunc(opts[i])(exts[i], chains[i])
		assert.Errorf(t, err, strconv.Itoa(i))
	})

	for _, handler := range extensions.AllHandlers {
		assert.NotNil(t, handler.ID())
		assert.NotNil(t, handler.NewHandlerFunc(nil))
	}
}

func TestHandlers_WithOptions(t *testing.T) {
	handlerFuncMap := extensions.AllHandlers.WithOptions(&extensions.Options{})
	for _, handler := range extensions.AllHandlers {
		id := handler.ID()
		require.NotNil(t, id)

		handleFunc, ok := handlerFuncMap[id]
		assert.True(t, ok)
		assert.NotNil(t, handleFunc)
	}

}
