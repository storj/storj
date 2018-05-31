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

	"github.com/stretchr/testify/assert"
)

var quickConfig = &quick.Config{
	Values: func(values []reflect.Value, r *rand.Rand) {
		randHex := fmt.Sprintf("%x", r.Uint32())
		values[0] = reflect.ValueOf(randHex)
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
	tlsFileOptions *TlsFileOptions
	before         func (*tlsFileOptionsTestCase) (error)
	after          func (*tlsFileOptionsTestCase) (error)
}

func TestEnsureAbsPath(t *testing.T) {
	f := func (val string) (_ bool) {
		opts := &TlsFileOptions{
			CertRelPath: fmt.Sprintf("%s.crt", val),
			KeyRelPath: fmt.Sprintf("%s.key", val),
		}

		opts.EnsureAbsPaths()

		if opts.CertAbsPath == "" && opts.KeyAbsPath == "" {
			quickLog("absolute path is empty string", opts, nil)
			return false
		}

		base := filepath.Base
		wrongCert :=  base(opts.CertAbsPath) != base(opts.CertRelPath)
		wrongKey :=  base(opts.CertAbsPath) != base(opts.CertRelPath)

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
  tempPath , err := ioutil.TempDir("", "TestGenerate")
  assert.NoError(t, err)
  defer os.RemoveAll(tempPath)

  f := func (val string) (_ bool) {
    basePath := filepath.Join(tempPath, val)
    certPath := fmt.Sprintf("%s.crt", basePath)
    keyPath := fmt.Sprintf("%s.key", basePath)

    opts := &TlsFileOptions{
      CertAbsPath: certPath,
      KeyAbsPath: keyPath,
      Create: true,
      Overwrite: false,
      Hosts: "127.0.0.1",
    }

    if err := opts.generate(); err != nil {
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

    // TODO: read files back in and compare to in-memory copies...

    return true
  }

  err = quick.Check(f, quickConfig)
  assert.NoError(t, err)
}

func TestEnsureExists_Create(t *testing.T) {
	tempPath , err := ioutil.TempDir("", "TestEnsureExists_Create")
	assert.NoError(t, err)
	defer os.RemoveAll(tempPath)

	f := func (val string) (_ bool) {
		basePath := filepath.Join(tempPath, val)
		certPath := fmt.Sprintf("%s.crt", basePath)
		keyPath := fmt.Sprintf("%s.key", basePath)

		opts := &TlsFileOptions{
			CertAbsPath: certPath,
			KeyAbsPath: keyPath,
			Create: true,
			Overwrite: false,
		}

		err := opts.EnsureExists(); if err != nil {
			quickLog("ensureExists err", opts, err)
			return false
		}

		fPaths := []string{certPath, keyPath}
		for _, fPath := range fPaths {
			_, err = os.Stat(fPath); if err != nil {
				quickLog("path doesn't exist", opts, nil)
				return false
			}
		}

		return true
	}

	err = quick.Check(f, quickConfig)

	assert.NoError(t, err)
}

func TestEnsureExists_NotExistError(t *testing.T) {
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