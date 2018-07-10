// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type tlsFileOptionsTestCase struct {
	tlsFileOptions *TLSHelper
	before         func(*tlsFileOptionsTestCase) error
	after          func(*tlsFileOptionsTestCase) error
}

func TestNewTLSFileOptions(t *testing.T) {
	opts, err := NewTLSHelper()
	assert.NoError(t, err)
	assert.NotEmpty(t, opts.cert)
	assert.NotEmpty(t, opts.cert.PrivateKey)
}

// func TestEnsureAbsPath(t *testing.T) {
// 	f := func(val string) (_ bool) {
// 		opts := &TLSHelper{
// 			RootCertRelPath: fmt.Sprintf("%s.root.cert", val),
// 			RootKeyRelPath:  fmt.Sprintf("%s.root.key", val),
// 			LeafCertRelPath: fmt.Sprintf("%s.leaf.cert", val),
// 			LeafKeyRelPath:  fmt.Sprintf("%s.leaf.key", val),
// 		}
//
// 		opts.EnsureAbsPaths()
//
// 		// TODO(bryanchriswhite) cleanup/refactor
// 		for _, requiredRole := range opts.requiredFiles() {
// 			for absPtr, role := range opts.pathRoleMap() {
// 				if role == requiredRole {
// 					if *absPtr == "" {
// 						msg := fmt.Sprintf("absolute path for %s is empty string", fileLabels[role])
// 						quickLog(msg, opts, nil)
// 						return false
// 					}
// 				}
// 			}
// 		}
//
// 		for _, requiredRole := range opts.requiredFiles() {
// 			for absPtr, role := range opts.pathRoleMap() {
// 				base := filepath.Base
// 				if role == requiredRole {
// 					relPath := opts.pathMap()[absPtr]
// 					if base(*absPtr) != base(relPath) {
// 						quickLog("basenames don't match", opts, nil)
// 						return false
// 					}
// 				}
// 			}
// 		}
//
// 		return true
// 	}
//
// 	err := quick.Check(f, quickConfig)
// 	assert.NoError(t, err)
// }

func TestGenerate(t *testing.T) {
	opts := &TLSHelper{}

	err := opts.generateTLS()
	assert.NoError(t, err)

	assert.NotNil(t, opts.cert)
	assert.NotNil(t, opts.cert.PrivateKey)

	err = VerifyPeerCertificate(opts.cert.Certificate, nil)
	assert.NoError(t, err)
}

// func TestLoadTLS(t *testing.T) {
// 	tempPath, err := ioutil.TempDir("", "TestLoadTLS")
// 	assert.NoError(t, err)
// 	defer os.RemoveAll(tempPath)
//
// 	f := func(val string) bool {
// 		var err error
//
// 		basePath := filepath.Join(tempPath, val)
// 		assert.NoError(t, err)
// 		defer os.RemoveAll(basePath)
//
// 		// Generate/write certs/keys to files
// 		generatedTLS, err := NewTLSHelper(
// 			basePath,
// 			basePath,
// 			true,
// 			true,
// 		)
//
// 		if err != nil {
// 			quickLog("NewTLSHelper error", nil, err)
// 			return false
// 		}
//
// 		loadedTLS, err := NewTLSHelper(
// 			basePath,
// 			basePath,
// 			false,
// 			false,
// 		)
//
// 		if err != nil {
// 			quickLog("NewTLSHelper error", nil, err)
// 			return false
// 		}
//
// 		if !certsMatch(
// 			generatedTLS.cert,
// 			loadedTLS.cert,
// 		) {
// 			return false
// 		}
//
// 		if !keysMatch(
// 			privKeyBytes(t, generatedTLS.cert.PrivateKey),
// 			privKeyBytes(t, loadedTLS.cert.PrivateKey),
// 		) {
// 			quickLog("keys don't match", nil, nil)
// 			return false
// 		}
//
// 		return true
// 	}
//
// 	err = quick.Check(f, quickConfig)
// 	assert.NoError(t, err)
// }

// func TestEnsureExists_Create(t *testing.T) {
// 	tempPath, err := ioutil.TempDir("", "TestEnsureExists_Create")
// 	assert.NoError(t, err)
// 	defer os.RemoveAll(tempPath)
//
// 	f := func(val string) bool {
// 		basePath := filepath.Join(tempPath, val)
// 		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
// 		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
// 		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
// 		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)
//
// 		opts := &TLSHelper{
// 			RootCertAbsPath: RootCertPath,
// 			RootKeyAbsPath:  RootKeyPath,
// 			LeafCertAbsPath: LeafCertPath,
// 			LeafKeyAbsPath:  LeafKeyPath,
// 			Create:          true,
// 			Overwrite:       false,
// 		}
//
// 		err := opts.EnsureExists()
// 		if err != nil {
// 			quickLog("ensureExists err", opts, err)
// 			return false
// 		}
//
// 		for _, requiredRole := range opts.requiredFiles() {
// 			for absPtr, role := range opts.pathRoleMap() {
// 				if role == requiredRole {
// 					if _, err = os.Stat(*absPtr); err != nil {
// 						quickLog("path doesn't exist", opts, nil)
// 						return false
// 					}
// 				}
// 			}
// 		}
//
// 		// TODO: check for *tls.Certificate and pubKey
//
// 		return true
// 	}
//
// 	err = quick.Check(f, quickConfig)
//
// 	assert.NoError(t, err)
// }
//
// func TestEnsureExists_Overwrite(t *testing.T) {
// 	tempPath, err := ioutil.TempDir("", "TestEnsureExists_Overwrite")
// 	assert.NoError(t, err)
// 	defer os.RemoveAll(tempPath)
//
// 	f := func(val string) (_ bool) {
// 		basePath := filepath.Join(tempPath, val)
// 		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
// 		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
// 		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
// 		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)
//
// 		checkFiles := func(opts *TLSHelper, checkSize bool) bool {
// 			for _, requiredRole := range opts.requiredFiles() {
// 				for absPtr, role := range opts.pathRoleMap() {
// 					if role == requiredRole {
// 						f, err := os.Stat(*absPtr)
//
// 						if err != nil {
// 							quickLog(fmt.Sprintf("%s path doesn't exist", *absPtr), opts, nil)
// 							return false
// 						}
//
// 						if checkSize && !(f.Size() > 0) {
// 							quickLog(fmt.Sprintf("%s has size 0", *absPtr), opts, nil)
// 							return false
// 						}
// 					}
// 				}
// 			}
//
// 			return true
// 		}
//
// 		requiredFiles := []string{
// 			RootCertPath,
// 			RootKeyPath,
// 			LeafCertPath,
// 			LeafKeyPath,
// 		}
//
// 		for _, path := range requiredFiles {
// 			if c, err := os.Create(path); err != nil {
// 				quickLog("", nil, errs.Wrap(err))
// 				return false
// 			} else {
// 				c.Close()
// 			}
// 		}
//
// 		opts := &TLSHelper{
// 			RootCertAbsPath: RootCertPath,
// 			RootKeyAbsPath:  RootKeyPath,
// 			LeafCertAbsPath: LeafCertPath,
// 			LeafKeyAbsPath:  LeafKeyPath,
// 			Create:          true,
// 			Overwrite:       true,
// 		}
//
// 		// Ensure files exist to be overwritten
// 		checkFiles(opts, false)
//
// 		if err := opts.EnsureExists(); err != nil {
// 			quickLog("ensureExists err", opts, err)
// 			return false
// 		}
//
// 		checkFiles(opts, true)
//
// 		return true
// 	}
//
// 	err = quick.Check(f, quickConfig)
// 	assert.NoError(t, err)
// }
//
// func TestEnsureExists_NotExistError(t *testing.T) {
// 	tempPath, err := ioutil.TempDir("", "TestEnsureExists_NotExistError")
// 	assert.NoError(t, err)
// 	defer os.RemoveAll(tempPath)
//
// 	f := func(val string) (_ bool) {
// 		basePath := filepath.Join(tempPath, val)
// 		RootCertPath := fmt.Sprintf("%s.root.cert", basePath)
// 		RootKeyPath := fmt.Sprintf("%s.root.key", basePath)
// 		LeafCertPath := fmt.Sprintf("%s.leaf.cert", basePath)
// 		LeafKeyPath := fmt.Sprintf("%s.leaf.key", basePath)
//
// 		opts := &TLSHelper{
// 			RootCertAbsPath: RootCertPath,
// 			RootKeyAbsPath:  RootKeyPath,
// 			LeafCertAbsPath: LeafCertPath,
// 			LeafKeyAbsPath:  LeafKeyPath,
// 			Create:          false,
// 			Overwrite:       false,
// 		}
//
// 		if err := opts.EnsureExists(); err != nil {
// 			if IsNotExist(err) {
// 				return true
// 			}
//
// 			quickLog("unexpected err", opts, err)
// 			return false
// 		}
//
// 		quickLog("didn't error but should've", opts, nil)
// 		return false
// 	}
//
// 	err = quick.Check(f, quickConfig)
//
// 	assert.NoError(t, err)
// }

func TestNewTLSConfig(t *testing.T) {
	opts, err := NewTLSHelper()
	assert.NoError(t, err)

	config := opts.NewTLSConfig(nil)
	assert.Equal(t, *opts.cert, config.Certificates[0])
}

// func privKeyBytes(t *testing.T, key crypto.PrivateKey) []byte {
// 	switch key.(type) {
// 	case *ecdsa.PrivateKey:
// 	default:
// 		quickLog("non-ecdsa private key", key, nil)
// 		panic("non-ecdsa private key")
// 	}
// 	ecKey := key.(*ecdsa.PrivateKey)
// 	b, err := x509.MarshalECPrivateKey(ecKey)
// 	assert.NoError(t, err)
//
// 	return b
// }
//
// func certsMatch(c1, c2 *tls.Certificate) bool {
// 	for i, cert := range c1.Certificate {
// 		if bytes.Compare(cert, c2.Certificate[i]) != 0 {
// 			return false
// 		}
// 	}
//
// 	return true
// }
//
// func keysMatch(k1, k2 []byte) bool {
// 	return bytes.Compare(k1, k2) == 0
// }
