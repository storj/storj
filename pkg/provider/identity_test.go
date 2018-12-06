// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/peertls"
)

func TestPeerIdentityFromCertChain(t *testing.T) {
	caKey, err := peertls.NewKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, caKey, caTemplate, nil)
	assert.NoError(t, err)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := peertls.NewKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)

	peerIdent, err := PeerIdentityFromCerts(leafCert, caCert, nil)
	assert.NoError(t, err)
	assert.Equal(t, caCert, peerIdent.CA)
	assert.Equal(t, leafCert, peerIdent.Leaf)
	assert.NotEmpty(t, peerIdent.ID)
}

func TestFullIdentityFromPEM(t *testing.T) {
	caKey, err := peertls.NewKey()
	assert.NoError(t, err)

	caTemplate, err := peertls.CATemplate()
	assert.NoError(t, err)

	caCert, err := peertls.NewCert(caKey, caKey, caTemplate, nil)
	assert.NoError(t, err)
	assert.NoError(t, err)
	assert.NotEmpty(t, caCert)

	leafTemplate, err := peertls.LeafTemplate()
	assert.NoError(t, err)

	leafKey, err := peertls.NewKey()
	assert.NoError(t, err)

	leafCert, err := peertls.NewCert(leafKey, caKey, leafTemplate, caTemplate)
	assert.NoError(t, err)
	assert.NotEmpty(t, leafCert)

	chainPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(leafCert.Raw)))
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(caCert.Raw)))

	leafECKey, ok := leafKey.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, leafECKey)

	leafKeyBytes, err := x509.MarshalECPrivateKey(leafECKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, leafKeyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(keyPEM, peertls.NewKeyBlock(leafKeyBytes)))

	fullIdent, err := FullIdentityFromPEM(chainPEM.Bytes(), keyPEM.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, leafCert.Raw, fullIdent.Leaf.Raw)
	assert.Equal(t, caCert.Raw, fullIdent.CA.Raw)
	assert.Equal(t, leafKey, fullIdent.Key)
}

func TestIdentityConfig_SaveIdentity(t *testing.T) {
	done, ic, fi, _ := tempIdentity(t)
	defer done()

	chainPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(fi.Leaf.Raw)))
	assert.NoError(t, pem.Encode(chainPEM, peertls.NewCertBlock(fi.CA.Raw)))

	privateKey, ok := fi.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	assert.NotEmpty(t, privateKey)

	keyBytes, err := x509.MarshalECPrivateKey(privateKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, keyBytes)

	keyPEM := bytes.NewBuffer([]byte{})
	assert.NoError(t, pem.Encode(keyPEM, peertls.NewKeyBlock(keyBytes)))

	err = ic.Save(fi)
	assert.NoError(t, err)

	if runtime.GOOS != "windows" {
		//TODO (windows): ignoring for windows due to different default permissions
		certInfo, err := os.Stat(ic.CertPath)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), certInfo.Mode())

		keyInfo, err := os.Stat(ic.KeyPath)
		assert.NoError(t, err)
		assert.Equal(t, os.FileMode(0600), keyInfo.Mode())
	}

	savedChainPEM, err := ioutil.ReadFile(ic.CertPath)
	assert.NoError(t, err)

	savedKeyPEM, err := ioutil.ReadFile(ic.KeyPath)
	assert.NoError(t, err)

	assert.Equal(t, chainPEM.Bytes(), savedChainPEM)
	assert.Equal(t, keyPEM.Bytes(), savedKeyPEM)
}

func tempIdentityConfig() (*IdentityConfig, func(), error) {
	tmpDir, err := ioutil.TempDir("", "storj-identity")
	if err != nil {
		return nil, nil, err
	}

	cleanup := func() { _ = os.RemoveAll(tmpDir) }

	return &IdentityConfig{
		CertPath: filepath.Join(tmpDir, "chain.pem"),
		KeyPath:  filepath.Join(tmpDir, "key.pem"),
	}, cleanup, nil
}

func tempIdentity(t *testing.T) (func(), *IdentityConfig, *FullIdentity, uint16) {
	// NB: known difficulty
	difficulty := uint16(12)

	chain := `-----BEGIN CERTIFICATE-----
MIIBQDCB56ADAgECAhB+u3d03qyW/ROgwy/ZsPccMAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIZrEPV/ExEkF0qUF0fJ3qSeGt5oFUX231v02NSUywcQ/Ve0
v3nHbmcJdjWBis2AkfL25mYDVC25jLl4tylMKumjPzA9MA4GA1UdDwEB/wQEAwIF
oDAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAK
BggqhkjOPQQDAgNIADBFAiEA2ZvsR0ncw4mHRIg2Isavd+XVEoMo/etXQRAkDy9n
wyoCIDykUsqjshc9kCrXOvPSN8GuO2bNoLu5C7K1GlE/HI2X
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBODCB4KADAgECAhAOcvhKe5TWT44LqFfgA1f8MAoGCCqGSM49BAMCMAAwIhgP
MDAwMTAxMDEwMDAwMDBaGA8wMDAxMDEwMTAwMDAwMFowADBZMBMGByqGSM49AgEG
CCqGSM49AwEHA0IABIZrEPV/ExEkF0qUF0fJ3qSeGt5oFUX231v02NSUywcQ/Ve0
v3nHbmcJdjWBis2AkfL25mYDVC25jLl4tylMKumjODA2MA4GA1UdDwEB/wQEAwIC
BDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MAoGCCqGSM49
BAMCA0cAMEQCIGAZfPT1qvlnkTacojTtP20ZWf6XbnSztJHIKlUw6AE+AiB5Vcjj
awRaC5l1KBPGqiKB0coVXDwhW+K70l326MPUcg==
-----END CERTIFICATE-----`

	key := `-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIKGjEetrxKrzl+AL1E5LXke+1ElyAdjAmr88/1Kx09+doAoGCCqGSM49
AwEHoUQDQgAEoLy/0hs5deTXZunRumsMkiHpF0g8wAc58aXANmr7Mxx9tzoIYFnx
0YN4VDKdCtUJa29yA6TIz1MiIDUAcB5YCA==
-----END EC PRIVATE KEY-----`

	ic, cleanup, err := tempIdentityConfig()
	assert.NoError(t, err)

	fi, err := FullIdentityFromPEM([]byte(chain), []byte(key))
	assert.NoError(t, err)

	return cleanup, ic, fi, difficulty
}

func TestIdentityConfig_LoadIdentity(t *testing.T) {
	done, ic, expectedFI, _ := tempIdentity(t)
	defer done()

	err := ic.Save(expectedFI)
	assert.NoError(t, err)

	fi, err := ic.Load()
	assert.NoError(t, err)
	assert.NotEmpty(t, fi)
	assert.NotEmpty(t, fi.Key)
	assert.NotEmpty(t, fi.Leaf)
	assert.NotEmpty(t, fi.CA)
	assert.NotEmpty(t, fi.ID.Bytes())

	assert.Equal(t, expectedFI.Key, fi.Key)
	assert.Equal(t, expectedFI.Leaf, fi.Leaf)
	assert.Equal(t, expectedFI.CA, fi.CA)
	assert.Equal(t, expectedFI.ID.Bytes(), fi.ID.Bytes())
}

func TestNewI(t *testing.T) {

}

func TestNodeID_Difficulty(t *testing.T) {
	done, _, fi, knownDifficulty := tempIdentity(t)
	defer done()

	difficulty, err := fi.ID.Difficulty()
	assert.NoError(t, err)
	assert.True(t, difficulty >= knownDifficulty)
}

func TestVerifyPeer(t *testing.T) {
	check := func(e error) {
		if !assert.NoError(t, e) {
			t.Fail()
		}
	}

	ca, err := newTestCA(context.Background())
	check(err)
	fi, err := ca.NewIdentity()
	check(err)

	err = peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)([][]byte{fi.Leaf.Raw, fi.CA.Raw}, nil)
	assert.NoError(t, err)
}
