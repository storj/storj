// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/testcontext"
	"storj.io/gateway-mt/pkg/auth"
	"storj.io/gateway-mt/pkg/authclient"
	"storj.io/gateway-mt/pkg/server"
	"storj.io/gateway-mt/pkg/trustedip"
	"storj.io/private/cfgstruct"
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
type EdgeTest func(t *testing.T, ctx *testcontext.Context, planet *EdgePlanet, browser *rod.Browser)

// Edge starts a testplanet together with auth service and gateway.
func Edge(t *testing.T, test EdgeTest) {
	edgehost := os.Getenv("STORJ_TEST_EDGE_HOST")
	if edgehost == "" {
		edgehost = "127.0.0.1"
	}

	// TODO: make address not hardcoded the address selection here may
	// conflict with some automatically bound address.
	startPort := randomRange(20000, 40000)
	authSvcAddr := net.JoinHostPort(edgehost, strconv.Itoa(startPort))
	authSvcAddrTLS := net.JoinHostPort(edgehost, strconv.Itoa(startPort+1))
	authSvcDrpcAddrTLS := net.JoinHostPort(edgehost, strconv.Itoa(startPort+2))

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				configureSatellite(log, index, config)
				// TODO: this should be dynamically set from the auth service
				config.Console.GatewayCredentialsRequestURL = "http://" + authSvcAddr
			},
		},
		NonParallel: true, // Note, do not remove this, because the code above uses same auth service addr.
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		access := planet.Uplinks[0].Access[planet.Satellites[0].ID()]

		gwConfig := server.Config{}
		cfgstruct.Bind(&pflag.FlagSet{}, &gwConfig, cfgstruct.UseTestDefaults())
		gwConfig.Server.Address = "127.0.0.1:0"
		gwConfig.Auth.BaseURL = "http://" + authSvcAddr
		gwConfig.Auth.Token = "super-secret"
		gwConfig.Auth.Timeout = 5 * time.Minute
		gwConfig.InsecureLogAll = true

		authClient := authclient.New(gwConfig.Auth)

		gateway, err := server.New(gwConfig, planet.Log().Named("gateway"),
			trustedip.NewListTrustAll(), []string{}, authClient, 16)
		require.NoError(t, err)

		defer ctx.Check(gateway.Close)

		certFile, keyFile, _, _ := createSelfSignedCertificateFile(t, edgehost)

		authConfig := auth.Config{
			Endpoint:          "http://" + gateway.Address(),
			AuthToken:         []string{"super-secret"},
			KVBackend:         "memory://",
			ListenAddr:        authSvcAddr,
			ListenAddrTLS:     authSvcAddrTLS,
			DRPCListenAddr:    net.JoinHostPort(edgehost, "0"),
			DRPCListenAddrTLS: authSvcDrpcAddrTLS,
			CertFile:          certFile.Name(),
			KeyFile:           keyFile.Name(),
		}
		for _, sat := range planet.Satellites {
			authConfig.AllowedSatellites = append(authConfig.AllowedSatellites, sat.NodeURL().String())
		}

		authPeer, err := auth.New(ctx, planet.Log().Named("auth"), authConfig, ctx.Dir("authservice"))
		require.NoError(t, err)

		defer ctx.Check(authPeer.Close)
		ctx.Go(func() error { return authPeer.Run(ctx) })
		require.NoError(t, waitForAddress(ctx, authSvcAddrTLS, 3*time.Second))
		require.NoError(t, waitForAddress(ctx, authSvcDrpcAddrTLS, 3*time.Second))

		ctx.Go(func() error {
			return gateway.Run(ctx)
		})
		require.NoError(t, waitForAddress(ctx, gateway.Address(), 3*time.Second))

		edgeCredentials, err := cmd.RegisterAccess(ctx, access, authSvcDrpcAddrTLS, false, certFile.Name())
		require.NoError(t, err)

		edge := &EdgePlanet{}
		edge.Planet = planet
		edge.Gateway.AccessKey = edgeCredentials.AccessKeyID
		edge.Gateway.SecretKey = edgeCredentials.SecretKey
		edge.Gateway.Addr = edgeCredentials.Endpoint
		edge.Auth.Addr = authSvcAddr

		Browser(t, ctx, planet, func(browser *rod.Browser) {
			test(t, ctx, edge, browser)
		})
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

func randomRange(low, high int) int {
	// this generates biased crypt random numbers
	// but it uses crypt/rand to avoid potentially
	// someone seeding math/rand.
	span := high - low
	return low + int(randomUint64()&0x7FFF_FFFF)%span
}

func randomUint64() uint64 {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic(err)
	}

	return binary.LittleEndian.Uint64(value[:])
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
