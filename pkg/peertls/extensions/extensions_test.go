// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package extensions_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"storj.io/storj/pkg/storj"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/extensions"
)

func TestHandlers_Register(t *testing.T) {
	var (
		handlers = extensions.HandlerFactories{}
		ids      []*extensions.ExtensionID
		opts     []*extensions.Options
		exts     []pkix.Extension
		chains   [][][]*x509.Certificate
	)

	for i := 0; i < 5; i++ {
		ids = append(ids, &extensions.ExtensionID{2, 999, 999, i})
		opts = append(opts, &extensions.Options{})
		exts = append(exts, pkix.Extension{Id: *ids[i]})

		_, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)
		chains = append(chains, identity.ToChains(chain))

		testHandler := extensions.NewHandlerFactory(
			ids[i],
			func(opt *extensions.Options) extensions.HandlerFunc {
				assert.Equal(t, opts[i], opt)
				assert.NotNil(t, opt)

				return func(ext pkix.Extension, chain [][]*x509.Certificate) error {
					assert.NotNil(t, ext)
					assert.Equal(t, exts[i], ext)

					assert.NotNil(t, ext.Id)
					assert.Equal(t, *ids[i], ext.Id)

					assert.NotNil(t, chain)
					assert.Equal(t, chains[i], chain)
					return errs.New(strconv.Itoa(i))
				}
			},
		)
		handlers.Register(testHandler)

		err = handlers[i].NewHandlerFunc(opts[i])(exts[i], chains[i])
		assert.Errorf(t, err, strconv.Itoa(i))
	}

	{ // test `extensions.AllHandlers`
		for _, handler := range extensions.AllHandlers {
			assert.NotNil(t, handler.ID())
			assert.NotNil(t, handler.NewHandlerFunc(nil))
		}
	}
}

func TestHandlers_WithOptions(t *testing.T) {
	var (
		handlers = extensions.HandlerFactories{}
		ids      []*extensions.ExtensionID
		opts     []*extensions.Options
		exts     []pkix.Extension
		chains   [][][]*x509.Certificate
	)

	for i := 0; i < 5; i++ {
		ids = append(ids, &extensions.ExtensionID{2, 999, 999, i})
		opts = append(opts, &extensions.Options{})
		exts = append(exts, pkix.Extension{Id: *ids[i]})

		_, chain, err := testpeertls.NewCertChain(2, storj.LatestIDVersion().Number)
		require.NoError(t, err)
		chains = append(chains, identity.ToChains(chain))

		testHandler := extensions.NewHandlerFactory(
			ids[i],
			func(opt *extensions.Options) extensions.HandlerFunc {
				assert.Equal(t, opts[i], opt)
				assert.NotNil(t, opt)

				return func(ext pkix.Extension, chain [][]*x509.Certificate) error {
					assert.NotNil(t, ext)
					assert.Equal(t, exts[i], ext)

					assert.NotNil(t, ext.Id)
					assert.Equal(t, *ids[i], ext.Id)

					assert.NotNil(t, chain)
					assert.Equal(t, chains[i], chain)
					return errs.New(strconv.Itoa(i))
				}
			},
		)
		handlers.Register(testHandler)

		handlerFuncMap := handlers.WithOptions(&extensions.Options{})

		id := handlers[i].ID()
		require.NotNil(t, id)

		handleFunc, ok := handlerFuncMap[id]
		assert.True(t, ok)
		assert.NotNil(t, handleFunc)
	}

	{ // test `extensions.AllHandlers`
		handlerFuncMap := extensions.AllHandlers.WithOptions(&extensions.Options{})
		for _, handler := range extensions.AllHandlers {
			id := handler.ID()
			require.NotNil(t, id)

			handleFunc, ok := handlerFuncMap[id]
			assert.True(t, ok)
			assert.NotNil(t, handleFunc)
		}
	}
}
