// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
)

var quickConfig = &quick.Config{
	Values: func(values []reflect.Value, r *rand.Rand) {
		randHex := fmt.Sprintf("%x", r.Uint32())
		values[0] = reflect.ValueOf(randHex)
	},
}

var quickTLSOptionsConfig = &quick.Config{
	Values: func(values []reflect.Value, r *rand.Rand) {
		for i := range [3]bool{} {
			randHex := fmt.Sprintf("%x", r.Uint32())
			values[i] = reflect.ValueOf(randHex)
		}

		for i := range [2]bool{} {
			randBool := r.Uint32()&0x01 != 0
			values[i+3] = reflect.ValueOf(randBool)
		}
	},
}

var quickLog = func(msg string, obj interface{}, err error) {
	if msg != "" {
		fmt.Printf("%s:\n", msg)
	}

	if obj != nil {
		fmt.Printf("obj: %v\n", obj)
	}

	if err != nil {
		fmt.Printf("%+v\n", err)
	}
}

type tlsFileOptionsTestCase struct {
	tlsFileOptions *TLSFileOptions
	before         func(*tlsFileOptionsTestCase) error
	after          func(*tlsFileOptionsTestCase) error
}

func TestNewTLSFileOptions(t *testing.T) {
	f := func(cert, key, hosts string, client, overwrite bool) (_ bool) {
		tempPath, err := ioutil.TempDir("", "TestNewTLSFileOptions")
		assert.NoError(t, err)
		defer os.RemoveAll(tempPath)

		certPath := fmt.Sprintf("%s.crt", filepath.Join(tempPath, cert))
		keyPath := fmt.Sprintf("%s.key", filepath.Join(tempPath, key))
		opts, err := NewTLSFileOptions(certPath, keyPath, hosts, client, true, overwrite)
		if !assert.NoError(t, err) {
			quickLog("", nil, err)
			return false
		}

		if !assert.NotEmpty(t, opts.CertAbsPath) {
			return false
		}
		if !assert.NotEmpty(t, opts.KeyAbsPath) {
			return false
		}
		if !assert.NotEmpty(t, opts.Certificate.PrivateKey) {
			return false
		}
		if !assert.NotEmpty(t, opts.Certificate) {
			return false
		}
		if !assert.Equal(t, opts.CertRelPath, certPath) {
			return false
		}
		if !assert.Equal(t, opts.KeyRelPath, keyPath) {
			return false
		}
		if !assert.Equal(t, opts.Hosts, hosts) {
			return false
		}
		if !assert.Equal(t, opts.Client, client) {
			return false
		}
		if !assert.Equal(t, opts.Overwrite, overwrite) {
			return false
		}

		return true
	}

	err := quick.Check(f, quickTLSOptionsConfig)
	assert.NoError(t, err)
}

func TestEnsureAbsPath(t *testing.T) {
	f := func(val string) (_ bool) {
		opts := &TLSFileOptions{
			CertRelPath: fmt.Sprintf("%s.crt", val),
			KeyRelPath:  fmt.Sprintf("%s.key", val),
		}

		opts.EnsureAbsPaths()

		if opts.CertAbsPath == "" && opts.KeyAbsPath == "" {
			quickLog("absolute path is empty string", opts, nil)
			return false
		}

		base := filepath.Base
		wrongCert := base(opts.CertAbsPath) != base(opts.CertRelPath)
		wrongKey := base(opts.CertAbsPath) != base(opts.CertRelPath)

		if wrongCert || wrongKey {
			quickLog("basenames don't match", opts, nil)
			return false
		}

		return true
	}

	err := quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func TestGenerate(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestGenerate")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		certPath := fmt.Sprintf("%s.crt", basePath)
		keyPath := fmt.Sprintf("%s.key", basePath)

		opts := &TLSFileOptions{
			CertAbsPath: certPath,
			KeyAbsPath:  keyPath,
			Create:      true,
			Overwrite:   false,
			Hosts:       "127.0.0.1",
		}

		if err := opts.generateServerTls(); err != nil {
			quickLog("", opts, err)
			return false
		}

		fPaths := map[string]string{"cert": certPath, "key": keyPath}
		for k, fPath := range fPaths {
			if _, err := os.Stat(fPath); err != nil {
				err = ErrNotExist.Wrap(err)
				quickLog(fmt.Sprintf("%s error", k), opts, err)
				return false
			}
		}

		loadedCert, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			quickLog("", opts, err)
			return false
		}

		privKeyBytes := func(key crypto.PrivateKey) []byte {
			switch key.(type) {
			case *ecdsa.PrivateKey:
			default:
				quickLog("non-ecdsa private key", key, nil)
				panic("non-ecdsa private key")
			}
			ecKey := key.(*ecdsa.PrivateKey)
			b, err := x509.MarshalECPrivateKey(ecKey)
			assert.NoError(t, err)

			return b
		}

		certsMatch := func(c1, c2 *tls.Certificate) bool {
			for i, cert := range c1.Certificate {
				if bytes.Compare(cert, c2.Certificate[i]) != 0 {
					return false
				}
			}

			k1 := privKeyBytes(c1.PrivateKey)
			k2 := privKeyBytes(c2.PrivateKey)

			if bytes.Compare(k1, k2) != 0 {
				return false
			}

			return true
		}

		return certsMatch(&loadedCert, opts.Certificate)
	}

	err = quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func TestEnsureExists_Create(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestEnsureExists_Create")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		certPath := fmt.Sprintf("%s.crt", basePath)
		keyPath := fmt.Sprintf("%s.key", basePath)

		opts := &TLSFileOptions{
			CertAbsPath: certPath,
			KeyAbsPath:  keyPath,
			Create:      true,
			Overwrite:   false,
			Hosts:       "127.0.0.1",
		}

		err := opts.EnsureExists()
		if err != nil {
			quickLog("ensureExists err", opts, err)
			return false
		}

		fPaths := []string{certPath, keyPath}
		for _, fPath := range fPaths {
			if _, err = os.Stat(fPath); err != nil {
				quickLog("path doesn't exist", opts, nil)
				return false
			}
		}

		return true
	}

	err = quick.Check(f, quickConfig)

	assert.NoError(t, err)
}

func TestEnsureExists_Overwrite(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestEnsureExists_Overwrite")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		certPath := fmt.Sprintf("%s.crt", basePath)
		keyPath := fmt.Sprintf("%s.key", basePath)
		fPaths := map[string]string{"cert": certPath, "key": keyPath}

		checkFiles := func(opts *TLSFileOptions, checkSize bool) bool {
			for k, fPath := range fPaths {
				f, err := os.Stat(fPath)

				if err != nil {
					quickLog(fmt.Sprintf("%s path doesn't exist", k), opts, nil)
					return false
				}

				if checkSize && !(f.Size() > 0) {
					quickLog(fmt.Sprintf("%s has size 0", k), opts, nil)
					return false
				}
			}

			return true
		}

		if c, err := os.Create(certPath); err != nil {
			quickLog("", nil, errs.Wrap(err))
			return false
		} else {
			c.Close()
		}

		if k, err := os.Create(keyPath); err != nil {
			quickLog("", nil, errs.Wrap(err))
			return false
		} else {
			k.Close()
		}

		opts := &TLSFileOptions{
			CertAbsPath: certPath,
			KeyAbsPath:  keyPath,
			Create:      true,
			Overwrite:   true,
			Hosts:       "127.0.0.1",
		}

		// Ensure files exist to be overwritten
		checkFiles(opts, false)

		if err := opts.EnsureExists(); err != nil {
			quickLog("ensureExists err", opts, err)
			return false
		}

		checkFiles(opts, true)

		return true
	}

	err = quick.Check(f, quickConfig)
	assert.NoError(t, err)
}

func TestEnsureExists_NotExistError(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestEnsureExists_NotExistError")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func(val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		certPath := fmt.Sprintf("%s.crt", basePath)
		keyPath := fmt.Sprintf("%s.key", basePath)

		opts := &TLSFileOptions{
			CertAbsPath: certPath,
			KeyAbsPath:  keyPath,
			Create:      false,
			Overwrite:   false,
		}

		if err := opts.EnsureExists(); err != nil {
			if IsNotExist(err) {
				return true
			}

			quickLog("unexpected err", opts, err)
			return false
		}

		quickLog("didn't error but should've", opts, nil)
		return false
	}

	err = quick.Check(f, quickConfig)

	assert.NoError(t, err)
}

func TestNewTLSConfig(t *testing.T) {
	tempPath, err := ioutil.TempDir("", "TestNewPeerTLS")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	basePath := filepath.Join(tempPath, "TestNewPeerTLS")
	certPath := fmt.Sprintf("%s.crt", basePath)
	keyPath := fmt.Sprintf("%s.key", basePath)

	opts, err := NewTLSFileOptions(
		certPath,
		keyPath,
		"127.0.0.1",
		false,
		true,
		false,
	)
	assert.NoError(t, err)

	config := opts.NewTLSConfig(nil)
	assert.Equal(t, *opts.Certificate, config.Certificates[0])
}
