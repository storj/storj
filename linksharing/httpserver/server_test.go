// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package httpserver

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/pkcrypto"
	"storj.io/common/testcontext"
)

var (
	testKey = mustSignerFromPEM(`-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgT8yIof+3qG3wQzXf
eAOcuTgWmgqXRnHVwKJl2g1pCb2hRANCAARWxVAPyT1BRs2hqiDuHlPXr1kVDXuw
7/a1USmgsVWiZ0W3JopcTbTMhvMZk+2MKqtWcc3gHF4vRDnHTeQl4lsx
-----END PRIVATE KEY-----
`)
	testCert = mustCreateLocalhostCert()
)

func TestServer(t *testing.T) {
	address := "localhost:0"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "OK")
	})
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{testCert.Raw},
				PrivateKey:  testKey,
			},
		},
	}

	testCases := []serverTestCase{
		{
			Name:    "missing address",
			Handler: handler,
			NewErr:  "server address is required",
		},
		{
			Name:    "bad address",
			Address: "this is no good",
			Handler: handler,
			NewErr:  "unable to listen on this is no good: listen tcp: address this is no good: missing port in address",
		},
		{
			Name:    "missing handler",
			Address: address,
			NewErr:  "server handler is required",
		},
		{
			Name:    "success via HTTP",
			Address: address,
			Handler: handler,
		},
		{
			Name:      "success via HTTPS",
			Address:   address,
			Handler:   handler,
			TLSConfig: tlsConfig,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.Name, func(t *testing.T) {
			ctx := testcontext.NewWithTimeout(t, time.Minute)
			defer ctx.Cleanup()

			s, ok := testCase.NewServer(t)
			if !ok {
				return
			}

			runCtx, cancel := context.WithCancel(ctx)
			ctx.Go(func() error {
				return s.Run(runCtx)
			})

			testCase.DoGet(t, s)
			cancel()
		})
	}
}

type serverTestCase struct {
	Name      string
	Address   string
	Handler   http.Handler
	TLSConfig *tls.Config
	NewErr    string
}

func (testCase *serverTestCase) NewServer(tb testing.TB) (*Server, bool) {
	s, err := New(zaptest.NewLogger(tb), Config{
		Name:      "test",
		Address:   testCase.Address,
		Handler:   testCase.Handler,
		TLSConfig: testCase.TLSConfig,
	})
	if testCase.NewErr != "" {
		require.EqualError(tb, err, testCase.NewErr)
		return nil, false
	}
	require.NoError(tb, err)
	return s, true
}

func (testCase *serverTestCase) DoGet(tb testing.TB, s *Server) {
	scheme := "http"
	client := &http.Client{}
	if testCase.TLSConfig != nil {
		scheme = "https"
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPoolFromCert(testCert),
			},
		}
	}

	resp, err := client.Get(fmt.Sprintf("%s://%s", scheme, s.Addr()))
	require.NoError(tb, err)
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(tb, resp.StatusCode, http.StatusOK)

	body, err := ioutil.ReadAll(resp.Body)
	assert.NoError(tb, err)
	assert.Equal(tb, "OK", string(body))
}

func mustSignerFromPEM(keyBytes string) crypto.Signer {
	key, err := pkcrypto.PrivateKeyFromPEM([]byte(keyBytes))
	if err != nil {
		panic(err)
	}
	return key.(crypto.Signer)
}

func mustCreateLocalhostCert() *x509.Certificate {
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(0),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, testKey.Public(), testKey)
	if err != nil {
		panic(err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		panic(err)
	}
	return cert
}

func certPoolFromCert(cert *x509.Certificate) *x509.CertPool {
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return pool
}
