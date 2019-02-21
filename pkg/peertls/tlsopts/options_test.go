// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts_test

import (
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/tlsopts"
)

var pregeneratedIdentities = testplanet.NewPregeneratedIdentities()

func TestNewOptions(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fi, err := testplanet.PregeneratedIdentity(0)
	require.NoError(t, err)

	whitelistPath := ctx.File("whitelist.pem")

	chainData, err := peertls.ChainBytes(fi.CA)
	assert.NoError(t, err)

	err = ioutil.WriteFile(whitelistPath, chainData, 0644)
	assert.NoError(t, err)

	cases := []struct {
		testID      string
		config      tlsopts.Config
		pcvFuncsLen int
	}{
		{
			"default",
			tlsopts.Config{},
			0,
		}, {
			"revocation processing",
			tlsopts.Config{
				RevocationDBURL: "bolt://" + ctx.File("revocation1.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation: true,
				},
			},
			2,
		}, {
			"ca whitelist verification",
			tlsopts.Config{
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
			},
			1,
		}, {
			"ca whitelist verification and whitelist signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				Extensions: peertls.TLSExtConfig{
					WhitelistSignedLeaf: true,
				},
			},
			2,
		}, {
			"revocation processing and whitelist verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation2.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation: true,
				},
			},
			3,
		}, {
			"revocation processing, whitelist, and signed leaf verification",
			tlsopts.Config{
				// NB: file doesn't actually exist
				PeerCAWhitelistPath: whitelistPath,
				UsePeerCAWhitelist:  true,
				RevocationDBURL:     "bolt://" + ctx.File("revocation3.db"),
				Extensions: peertls.TLSExtConfig{
					Revocation:          true,
					WhitelistSignedLeaf: true,
				},
			},
			3,
		},
	}

	for _, c := range cases {
		t.Log(c.testID)
		opts, err := tlsopts.NewOptions(fi, c.config)
		assert.NoError(t, err)
		assert.True(t, reflect.DeepEqual(fi, opts.Ident))
		assert.Equal(t, c.config, opts.Config)
		assert.Len(t, opts.PCVFuncs, c.pcvFuncsLen)
	}
}
