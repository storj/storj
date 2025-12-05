// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/cfgstruct"
	"storj.io/common/errs2"
	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/edge/pkg/auth"
	"storj.io/edge/pkg/auth/badgerauth"
	"storj.io/edge/pkg/authclient"
	"storj.io/edge/pkg/server"
	"storj.io/edge/pkg/trustedip"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

// EdgePlanet contains defaults for testplanet with Edge.
type EdgePlanet struct {
	*testplanet.Planet

	Gateway struct {
		AccessKey string
		SecretKey string

		Addr string
	}

	Auth struct {
		Addr string
	}
}

// EdgeTest defines common args for edge testing.
type EdgeTest func(t *testing.T, ctx *testcontext.Context, planet *EdgePlanet)

var counter int64

// Edge starts a new test which includes edge services.
func Edge(t *testing.T, test EdgeTest) {
	edgehost := os.Getenv("STORJ_TEST_EDGE_HOST")
	if edgehost == "" {
		edgehost = "127.0.0.1"
	}
	authSvcAddr := fmt.Sprintf("%s:1100%d", edgehost, atomic.AddInt64(&counter, 1))
	authSvcAddrTLS := fmt.Sprintf("%s:1100%d", edgehost, atomic.AddInt64(&counter, 1))
	authSvcDrpcAddrTLS := fmt.Sprintf("%s:1100%d", edgehost, atomic.AddInt64(&counter, 1))
	gwAddr := fmt.Sprintf("%s:1100%d", edgehost, atomic.AddInt64(&counter, 1))

	certFile, keyFile, _, _ := createSelfSignedCertificateFile(t, edgehost)

	testplanet.Run(t, testplanet.Config{
		Timeout:        15 * time.Minute,
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				configureSatellite(log, index, config)
				config.Console.GatewayCredentialsRequestURL = "http://" + authSvcAddr
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		gwConfig := server.Config{}
		cfgstruct.Bind(&pflag.FlagSet{}, &gwConfig, cfgstruct.UseTestDefaults())

		gwConfig.Server.Address = gwAddr
		gwConfig.Auth.BaseURL = "http://" + authSvcAddr
		gwConfig.InsecureLogAll = true
		authClient := authclient.New(gwConfig.Auth)

		gateway, err := server.New(gwConfig, zaptest.NewLogger(t).Named("gateway"), trustedip.NewListTrustAll(), []string{"*"}, authClient, 10)
		require.NoError(t, err)

		defer ctx.Check(gateway.Close)

		authConfig := auth.Config{
			Endpoint:          "http://" + gateway.Address(),
			AuthToken:         []string{"super-secret"},
			POSTSizeLimit:     4 * memory.KiB,
			AllowedSatellites: []string{planet.Satellites[0].NodeURL().String()},
			KVBackend:         "badger://",
			ListenAddr:        authSvcAddr,
			ListenAddrTLS:     authSvcAddrTLS,
			DRPCListenAddr:    net.JoinHostPort(edgehost, "0"),
			DRPCListenAddrTLS: authSvcDrpcAddrTLS,
			ProxyAddrTLS:      "127.0.0.1:0",
			CertFile:          certFile.Name(),
			KeyFile:           keyFile.Name(),
			Node: badgerauth.Config{
				FirstStart: true,
			},
		}
		authService, err := auth.New(ctx, zaptest.NewLogger(t).Named("auth"), authConfig, fpath.ApplicationDir("storj", "authservice"))
		require.NoError(t, err)

		// auth peer needs to be canceled to shut the servers down.
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		ctx.Go(func() error {
			defer ctx.Check(authService.Close)
			return errs2.IgnoreCanceled(authService.Run(cancelCtx))
		})

		require.NoError(t, waitForAuthSvcStart(ctx, authClient, time.Second))

		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]
		edgeCredentials, err := cmd.RegisterAccess(ctx, access, authSvcDrpcAddrTLS, false, certFile.Name())
		require.NoError(t, err)

		ctx.Go(func() error {
			return gateway.Run(cancelCtx)
		})
		require.NoError(t, waitForAddress(ctx, gwAddr, 5*time.Second))

		edge := &EdgePlanet{}
		edge.Planet = planet
		edge.Gateway.AccessKey = edgeCredentials.AccessKeyID
		edge.Gateway.SecretKey = edgeCredentials.SecretKey
		edge.Gateway.Addr = edgeCredentials.Endpoint
		edge.Auth.Addr = authSvcAddr

		test(t, ctx, edge)
	})
}

func createSelfSignedCertificateFile(t *testing.T, hostname string) (certFile *os.File, keyFile *os.File, certificatePEM []byte, privateKeyPEM []byte) {
	certificatePEM, privateKeyPEM = createSelfSignedCertificate(t, hostname)

	certFile, err := os.CreateTemp(os.TempDir(), "*-cert.pem")
	require.NoError(t, err)
	_, err = certFile.Write(certificatePEM)
	require.NoError(t, err)

	keyFile, err = os.CreateTemp(os.TempDir(), "*-key.pem")
	require.NoError(t, err)
	_, err = keyFile.Write(privateKeyPEM)
	require.NoError(t, err)

	return certFile, keyFile, certificatePEM, privateKeyPEM
}

func createSelfSignedCertificate(t *testing.T, hostname string) (certificatePEM []byte, privateKeyPEM []byte) {
	notAfter := time.Now().Add(1 * time.Minute)

	var ips []net.IP
	ip := net.ParseIP(hostname)
	if ip != nil {
		ips = []net.IP{ip}
	}

	// first create a server certificate
	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName: hostname,
		},
		DNSNames:              []string{hostname},
		IPAddresses:           ips,
		SerialNumber:          big.NewInt(1337),
		BasicConstraintsValid: false,
		IsCA:                  true,
		NotAfter:              notAfter,
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	certificateDERBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privateKey.PublicKey,
		privateKey,
	)
	require.NoError(t, err)

	certificatePEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDERBytes})

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	privateKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes})

	return certificatePEM, privateKeyPEM
}

// waitForAddress will monitor starting when we are able to start the process.
func waitForAddress(ctx context.Context, address string, maxStartupWait time.Duration) error {
	defer mon.Task()(&ctx)(nil)

	start := time.Now()
	for time.Since(start) < maxStartupWait {
		if tryConnect(ctx, address) {
			return ctx.Err()
		}

		// wait a bit before retrying to reduce load
		if !sync2.Sleep(ctx, 50*time.Millisecond) {
			return ctx.Err()
		}
	}
	return fmt.Errorf("did not start in required time %v", maxStartupWait)
}

// tryConnect will try to connect to the process public address.
func tryConnect(ctx context.Context, address string) bool {
	defer mon.Task()(&ctx)(nil)

	dialer := net.Dialer{}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	// write empty byte slice to trigger refresh on connection
	_, _ = conn.Write([]byte{})
	// ignoring errors, because we only care about being able to connect
	_ = conn.Close()
	return true
}

func waitForAuthSvcStart(ctx context.Context, authClient *authclient.AuthClient, maxStartupWait time.Duration) error {
	for start := time.Now(); ; {
		_, err := authClient.GetHealthLive(ctx)
		if err == nil {
			return nil
		}

		// wait a bit before retrying to reduce load
		time.Sleep(50 * time.Millisecond)
		if time.Since(start) > maxStartupWait {
			return errs.New("exceeded maxStartupWait duration")
		}
	}
}
