package utils

import (
	"os"
	"testing"
	"testing/quick"
	"fmt"
	"io/ioutil"
	"reflect"
	"math/rand"
	"path/filepath"

	"github.com/zeebo/errs"
	"github.com/stretchr/testify/assert"
)

var quickConfig = &quick.Config{
	Values: func(values []reflect.Value, r *rand.Rand) {
		randHex := fmt.Sprintf("%x", r.Uint32())
		values[0] = reflect.ValueOf(randHex)
	},
}

var quickLog = func(msg string, obj interface{}) {
fmt.Printf("%s:\n%v\n", msg, obj)
}


type tlsFileOptionsTestCase struct {
	tlsFileOptions *TlsFileOptions
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
		opts := &TlsFileOptions{
			CertRelPath: fmt.Sprintf("%s.crt", val),
			KeyRelPath: fmt.Sprintf("%s.key", val),
		}

		opts.EnsureAbsPaths()

		if opts.CertAbsPath == "" && opts.KeyAbsPath == "" {
			quickLog("absolute path is empty string", opts)
			return false
		}

		base := filepath.Base
		wrongCert :=  base(opts.CertAbsPath) != base(opts.CertRelPath)
		wrongKey :=  base(opts.CertAbsPath) != base(opts.CertRelPath)

		if wrongCert || wrongKey {
			quickLog("basenames don't match", opts)
			return false
		}

		return true
	}

	err := quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func TestEnsureExistsError(t *testing.T) {
	tempPath , err := ioutil.TempDir("", "TestEnsureExistsError")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func (val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		certPath := fmt.Sprintf("%s.crt", basePath)
		keyPath := fmt.Sprintf("%s.key", basePath)

		opts := &TlsFileOptions{
			CertAbsPath: certPath,
			KeyAbsPath: keyPath,
			Create: false,
			Overwrite: false,
		}

		err := opts.EnsureExists(); if err != nil {
			quickLog("ensureExists err", struct {*TlsFileOptions; error}{
				opts,
				err,
			})
			return false
		}

		fPaths := []string{certPath, keyPath}
		for _, fPath := range fPaths {
			_, err = os.Stat(fPath); if err != nil {
				quickLog("path doesn't exist", opts)
				return false
			}
		}

		return true
	}

	err = quick.Check(f, quickConfig)

	assert.NoError(t, err)
}

func TestTlsFileOptions(t *testing.T) {
	cases := []tlsFileOptionsTestCase{
		{
			// generate cert/key with given filename
			tlsFileOptions: &TlsFileOptions{
				CertRelPath: "./non-existent.cert",
				KeyRelPath:  "./non-existent.key",
			},
			before: ensureRemoved,
			after: ensureRemoved,
		},
		{
			// use defaults
			tlsFileOptions: &TlsFileOptions{},
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
