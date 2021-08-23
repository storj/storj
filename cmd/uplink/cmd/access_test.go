// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information

package cmd_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	"storj.io/storj/cmd/uplink/cmd"
	"storj.io/uplink"
	"storj.io/uplink/edge"
)

const testAccess = "12edqrJX1V243n5fWtUrwpMQXL8gKdY2wbyqRPSG3rsA1tzmZiQjtCyF896egifN2C2qdY6g5S1t6e8iDhMUon9Pb7HdecBFheAcvmN8652mqu8hRx5zcTUaRTWfFCKS2S6DHmTeqPUHJLEp6cJGXNHcdqegcKfeahVZGP4rTagHvFGEraXjYRJ3knAcWDGW6BxACqogEWez6r274JiUBfs4yRSbRNRqUEURd28CwDXMSHLRKKA7TEDKEdQ"

func TestRegisterAccess(t *testing.T) {
	ctx := testcontext.NewWithTimeout(t, 5*time.Second)
	defer ctx.Cleanup()

	server := DRPCServerMock{}

	cancelCtx, authCancel := context.WithCancel(ctx)
	defer authCancel()
	port, certificatePEM := startMockAuthService(cancelCtx, ctx, t, &server)
	caFile := ctx.File("cert.pem")
	err := os.WriteFile(caFile, certificatePEM, os.FileMode(0600))
	require.NoError(t, err)

	url := "https://localhost:" + strconv.Itoa(port)

	// make sure we get back things
	access, err := uplink.ParseAccess(testAccess)
	require.NoError(t, err)
	credentials, err := cmd.RegisterAccess(ctx, access, url, true, caFile)
	require.NoError(t, err)
	assert.Equal(t,
		&edge.Credentials{
			AccessKeyID: "l5pucy3dmvzxgs3fpfewix27l5pq",
			SecretKey:   "l5pvgzldojsxis3fpfpv6x27l5pv6x27l5pv6x27l5pv6",
			Endpoint:    "https://gateway.example",
		},
		credentials)
}

func TestAccessImport(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const testAccess = "12edqwjdy4fmoHasYrxLzmu8Ubv8Hsateq1LPYne6Jzd64qCsYgET53eJzhB4L2pWDKBpqMowxt8vqLCbYxu8Qz7BJVH1CvvptRt9omm24k5GAq1R99mgGjtmc6yFLqdEFgdevuQwH5yzXCEEtbuBYYgES8Stb1TnuSiU3sa62bd2G88RRgbTCtwYrB8HZ7CLjYWiWUphw7RNa3NfD1TW6aUJ6E5D1F9AM6sP58X3D4H7tokohs2rqCkwRT"

	uplinkExe := ctx.Compile("storj.io/storj/cmd/uplink")

	output, err := exec.Command(uplinkExe, "--config-dir", ctx.Dir("uplink"), "import", testAccess).CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
}

type DRPCServerMock struct {
	pb.DRPCEdgeAuthServer
}

func (g *DRPCServerMock) RegisterAccess(context.Context, *pb.EdgeRegisterAccessRequest) (*pb.EdgeRegisterAccessResponse, error) {
	return &pb.EdgeRegisterAccessResponse{
		AccessKeyId: "l5pucy3dmvzxgs3fpfewix27l5pq",
		SecretKey:   "l5pvgzldojsxis3fpfpv6x27l5pv6x27l5pv6x27l5pv6",
		Endpoint:    "https://gateway.example",
	}, nil
}

func startMockAuthService(cancelCtx context.Context, testCtx *testcontext.Context, t *testing.T, srv pb.DRPCEdgeAuthServer) (port int, certificatePEM []byte) {
	certificatePEM, privateKeyPEM := createSelfSignedCertificate(t, "localhost")

	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	require.NoError(t, err)

	serverTLSConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
	}

	drpcListener, err := tls.Listen("tcp", "127.0.0.1:0", serverTLSConfig)
	require.NoError(t, err)

	port = drpcListener.Addr().(*net.TCPAddr).Port

	mux := drpcmux.New()
	err = pb.DRPCRegisterEdgeAuth(mux, srv)
	require.NoError(t, err)

	server := drpcserver.New(mux)
	testCtx.Go(func() error {
		return server.Serve(cancelCtx, drpcListener)
	})

	return port, certificatePEM
}

func createSelfSignedCertificate(t *testing.T, hostname string) (certificatePEM []byte, privateKeyPEM []byte) {
	notAfter := time.Now().Add(1 * time.Minute)

	// first create a server certificate
	template := x509.Certificate{
		Subject: pkix.Name{
			CommonName: hostname,
		},
		DNSNames:              []string{hostname},
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
