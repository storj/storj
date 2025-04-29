// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tracker

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

func TestTracker(t *testing.T) {
	ctx := testcontext.New(t)

	nodeID1 := testrand.NodeID()
	nodeID2 := testrand.NodeID()
	nodeID3 := testrand.NodeID()

	db := &overlay.Mockdb{
		Reputable: []*nodeselection.SelectedNode{
			{
				ID: nodeID1,
				Tags: nodeselection.NodeTags{
					{
						Name:   "dc",
						Value:  []byte("dc1"),
						Signer: storj.NodeID{},
					},
					{
						Name:   "server",
						Value:  []byte("s1"),
						Signer: storj.NodeID{},
					},
					{
						Name:   "instance",
						Value:  []byte("storagenode1"),
						Signer: storj.NodeID{},
					},
				},
			},
			{
				ID: nodeID2,
				Tags: nodeselection.NodeTags{
					{
						Name:   "dc",
						Value:  []byte("dc1"),
						Signer: storj.NodeID{},
					},
					{
						Name:   "server",
						Value:  []byte("s1"),
						Signer: storj.NodeID{},
					},
					{
						Name:   "instance",
						Value:  []byte("storagenode2"),
						Signer: storj.NodeID{},
					},
				},
			},
			{
				ID: nodeID3,
				Tags: nodeselection.NodeTags{
					{
						Name:   "dc",
						Value:  []byte("dc1"),
						Signer: storj.NodeID{},
					},
					{
						Name:   "server",
						Value:  []byte("s1"),
						Signer: storj.NodeID{},
					},
					{
						Name:   "instance",
						Value:  []byte("storagenode3"),
						Signer: storj.NodeID{},
					},
				},
			},
		},
	}

	host := "127.0.0.1"
	if hostlist := os.Getenv("STORJ_TEST_HOST"); hostlist != "" {
		host, _, _ = strings.Cut(hostlist, ";")
	}

	certDir := t.TempDir()
	certPath, keyPath := generateCert(t, host, certDir)

	p, err := NewPrometheusStub(host, certPath, keyPath, "foo", "bar")
	require.NoError(t, err)
	defer func() {
		_ = p.Close()
	}()

	go func() {
		_ = p.Run()
	}()

	logger := zaptest.NewLogger(t)

	tracker, err := NewPrometheusTracker(logger, db, PrometheusTrackerConfig{
		URL:        "https://" + p.Addr(),
		CaCertPath: certPath,
		Username:   "foo",
		Password:   "bar",
		Query:      "response_time{}",
		Labels: []string{
			"datacenter",
			"host",
			"instance",
		},
		Attributes: []string{
			"tag:dc",
			"tag:server",
			"tag:instance",
		},
	})
	require.NoError(t, err)

	go func() {
		err := tracker.Run(ctx)
		if err != nil {
			t.Log(err)
		}
	}()

	score := tracker.Get(storj.NodeID{})
	require.Equal(t, 0.5, score(&nodeselection.SelectedNode{
		ID: nodeID1,
	}))
	require.Equal(t, 0.8, score(&nodeselection.SelectedNode{
		ID: nodeID2,
	}))
	require.True(t, math.IsNaN(score(&nodeselection.SelectedNode{
		ID: nodeID3,
	})))
}

func generateCert(t *testing.T, host, dir string) (string, string) {
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	require.NoError(t, err)

	cert := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Storj labs"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 1 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP(host)},
		DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &cert, &cert, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)

	certOut, err := os.Create(certPath)
	require.NoError(t, err)
	defer func() {
		_ = certOut.Close()
	}()
	err = pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	require.NoError(t, err)

	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	keyOut, err := os.Create(keyPath)
	require.NoError(t, err)
	defer func() {
		_ = keyOut.Close()
	}()
	require.NoError(t, err)
	err = pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})
	require.NoError(t, err)

	return certPath, keyPath
}

type PrometheusStub struct {
	listener net.Listener
	cert     string
	key      string
	username string
	password string
}

func NewPrometheusStub(host string, cert, key string, username string, password string) (*PrometheusStub, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return &PrometheusStub{
		listener: listener,
		cert:     cert,
		key:      key,
		username: username,
		password: password,
	}, nil
}

func (p *PrometheusStub) Addr() string {
	return p.listener.Addr().String()
}

func (p *PrometheusStub) Run() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", p.handleRequest)

	server := &http.Server{
		Handler: p.basicAuth(mux),
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}
	return server.ServeTLS(p.listener, p.cert, p.key)
}

func (p *PrometheusStub) handleRequest(writer http.ResponseWriter, request *http.Request) {
	response := `
{
   "status" : "success",
   "data" : {
      "resultType" : "vector",
      "result" : [
         {
            "metric" : {
               "__name__" : "response_time", "datacenter" : "dc1", "host" : "s1", "instance" : "storagenode1"
            },
            "value": [ 1435781451.781, "0.5" ]
         },
         {
            "metric" : {
               "__name__" : "response_time", "job" : "node", "datacenter" : "dc1", "host" : "s1", "instance" : "storagenode2"
            },
            "value" : [ 1435781451.781, "0.8" ]
         }
      ]
   }
}
`

	_, _ = fmt.Fprintln(writer, response)
}

func (p *PrometheusStub) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		payload, _ := base64.StdEncoding.DecodeString(auth[len("Basic "):])
		pair := string(payload)
		if pair != p.username+":"+p.password {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *PrometheusStub) Close() error {
	return p.listener.Close()
}
