package utils

import (
	"os"
	"testing"

	"github.com/zeebo/errs"
	"github.com/stretchr/testify/assert"
	"testing/quick"
	"fmt"
)

type tlsFileOptionsTestCase struct {
	tlsFileOptions *TlsFileOpions
	before         func (*tlsFileOptionsTestCase) (error)
	after          func (*tlsFileOptionsTestCase) (error)
}

func ensureRemoved(c *tlsFileOptionsTestCase) (_ error) {
	opts := c.tlsFileOptions
	err := opts.EnsureAbsPaths(); if err != nil {
		return err
	}

	fPaths := []string{opts.CertAbsPath, opts.KeyAbsPath}
	for _, fPath := range fPaths {
		err := os.Remove(fPath); if err != nil {
			return errs.New(err.Error())
		}
	}

	return nil
}

func TestEnsureAbsPath(t *testing.T) {
	f := func (val string) (_ bool) {
		opts := &TlsFileOpions{
			CertRelPath: fmt.Sprintf("%s.crt", val),
			KeyRelPath: fmt.Sprintf("%s.key", val),
		}

		opts.EnsureAbsPaths()

		return opts.CertAbsPath != "" && opts.KeyAbsPath != ""
	}

	err := quick.Check(f, nil)
	assert.NoError(t, err)
}

func TestTlsFileOptions(t *testing.T) {
	cases := []tlsFileOptionsTestCase{
		{
			// generate cert/key with given filename
			tlsFileOptions: &TlsFileOpions{
				CertRelPath: "./non-existent.cert",
				KeyRelPath:  "./non-existent.key",
			},
			before: ensureRemoved,
			after: ensureRemoved,
		},
		{
			// use defaults
			tlsFileOptions: &TlsFileOpions{},
			after:          ensureRemoved,
		},
	}

	for _, c := range cases {
		opts := c.tlsFileOptions
		err := opts.EnsureExists(); if err != nil {
			assert.NoError(t, err)
		}

		assert.NotEqual(t, opts.CertAbsPath, "certAbsPath is an empty string")
		assert.NotEqual(t, opts.KeyAbsPath, "keyAbsPath is an empty string")

		fPaths := []string{opts.CertAbsPath, opts.KeyAbsPath}
		for _, fPath := range fPaths {
			_, err := os.Stat(fPath)
			assert.NoError(t, err)
		}
	}
}
