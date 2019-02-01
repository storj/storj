// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"bytes"
	"crypto/x509/pkix"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testpeertls"
	"storj.io/storj/pkg/peertls"
)

func TestRevocationDB_Get(t *testing.T) {
	tmp, err := ioutil.TempDir("", "TestRevocationDB_Get")
	defer func() { _ = os.RemoveAll(tmp) }()

	// NB: key indices are reversed as compared to chain indices
	keys, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	ext, err := peertls.NewRevocationExt(keys[0], chain[peertls.LeafIndex])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	revDB, err := NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	var rev *peertls.Revocation
	t.Run("missing key", func(t *testing.T) {
		rev, err = revDB.Get(chain)
		assert.NoError(t, err)
		assert.Nil(t, rev)
	})

	nodeID, err := NodeIDFromKey(chain[peertls.CAIndex].PublicKey)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	err = revDB.DB.Put(nodeID.Bytes(), ext.Value)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	t.Run("existing key", func(t *testing.T) {
		rev, err = revDB.Get(chain)
		assert.NoError(t, err)

		revBytes, err := rev.Marshal()
		assert.NoError(t, err)
		assert.True(t, bytes.Equal(ext.Value, revBytes))
	})
}

func TestRevocationDB_Put(t *testing.T) {
	tmp, err := ioutil.TempDir("", "TestRevocationDB_Put")
	defer func() { _ = os.RemoveAll(tmp) }()

	// NB: key indices are reversed as compared to chain indices
	keys, chain, err := testpeertls.NewCertChain(2)
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	olderExt, err := peertls.NewRevocationExt(keys[0], chain[peertls.LeafIndex])
	assert.NoError(t, err)

	time.Sleep(1 * time.Second)
	ext, err := peertls.NewRevocationExt(keys[0], chain[peertls.LeafIndex])
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	time.Sleep(1 * time.Second)
	newerExt, err := peertls.NewRevocationExt(keys[0], chain[peertls.LeafIndex])
	assert.NoError(t, err)

	revDB, err := NewRevocationDBBolt(filepath.Join(tmp, "revocations.db"))
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	cases := []struct {
		testID   string
		ext      pkix.Extension
		errClass *errs.Class
		err      error
	}{
		{
			"new key",
			ext,
			nil,
			nil,
		},
		{
			"existing key - older timestamp",
			olderExt,
			&peertls.ErrExtension,
			peertls.ErrRevocationTimestamp,
		},
		{
			"existing key - newer timestamp",
			newerExt,
			nil,
			nil,
		},
		// TODO(bryanchriswhite): test empty/garbage cert/timestamp/sig
	}

	for _, c := range cases {
		t.Run(c.testID, func(t2 *testing.T) {
			if !assert.NotNil(t, c.ext) {
				t2.Fail()
				t.FailNow()
			}
			err = revDB.Put(chain, c.ext)
			if c.errClass != nil {
				assert.True(t, c.errClass.Has(err))
			}
			if c.err != nil {
				assert.Equal(t, c.err, err)
			}

			if c.err == nil && c.errClass == nil {
				if !assert.NoError(t2, err) {
					t2.Fail()
					t.FailNow()
				}
				func(t2 *testing.T, ext pkix.Extension) {
					nodeID, err := NodeIDFromKey(chain[peertls.CAIndex].PublicKey)
					if !assert.NoError(t2, err) {
						t2.FailNow()
					}

					revBytes, err := revDB.DB.Get(nodeID.Bytes())
					if !assert.NoError(t2, err) {
						t2.FailNow()
					}

					rev := new(peertls.Revocation)
					err = rev.Unmarshal(revBytes)
					assert.NoError(t2, err)
					assert.True(t2, bytes.Equal(ext.Value, revBytes))
				}(t2, c.ext)
			}
		})
	}
}
