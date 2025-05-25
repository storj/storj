// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/drpc/drpcmux"
	"storj.io/drpc/drpcserver"
	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

const testAPIKey = "13Yqe3oHi5dcnGhMu2ru3cmePC9iEYv6nDrYMbLRh4wre1KtVA9SFwLNAuuvWwc43b9swRsrfsnrbuTHQ6TJKVt4LjGnaARN9PhxJEu"

func TestShare(t *testing.T) {
	t.Run("share requires prefix", func(t *testing.T) {
		ultest.Setup(uplinkcli.Commands).Fail(t, "share")
	})

	t.Run("share default access", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	t.Run("share access with --readonly", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "--readonly", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	t.Run("share access with --disallow-lists", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "--disallow-lists", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Disallowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	t.Run("share access with --disallow-reads", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "--disallow-reads", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Disallowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	t.Run("share access with --writeonly", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		result := state.Fail(t, "share", "--writeonly", "sj://some/prefix")

		require.Equal(t, "permission is empty", result.Err.Error())
	})

	t.Run("share access with --public", func(t *testing.T) {
		// Can't run this scenario because AuthService is not running in testplanet.
		// If necessary we can mock AuthService like in https://github.com/storj/uplink/blob/main/testsuite/edge_test.go
		t.Skip("No AuthService available in testplanet")
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "--public", "--not-after=none", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	t.Run("share access with --not-after", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "--not-after", "2022-01-01T15:01:01-01:00", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : 2022-01-01 16:01:01
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	t.Run("share access with --max-object-ttl", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		state.Succeed(t, "share", "--max-object-ttl", "720h", "--readonly=false", "sj://some/prefix").RequireStdoutGlob(t, `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Allowed
			Lists        : Allowed
			Deletes      : Allowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : 720h0m0s
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
		`)
	})

	ctx, cancelCtx := context.WithCancel(testcontext.New(t))
	defer cancelCtx()

	mockServer := startDRPCAuthMockServer(ctx, t, drpcAuthMockServerConfig{})
	authAddr := mockServer.addr()

	apiKey, err := macaroon.ParseAPIKey(testAPIKey)
	require.NoError(t, err)

	encAccess := grant.NewEncryptionAccessWithDefaultKey(&storj.Key{})
	access, err := (&grant.Access{
		SatelliteAddress: "12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S@us1.storj.io:7777",
		APIKey:           apiKey,
		EncAccess:        encAccess,
	}).Serialize()
	require.NoError(t, err)

	t.Run("share access with --dns and --tls", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		expected := `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
			========== GATEWAY CREDENTIALS ===========================================================
			Access Key ID: accesskeyid
			Secret Key   : secretkey
			Endpoint     : endpoint
			Public Access: true
			=========== DNS INFO =====================================================================
			Remember to update the $ORIGIN with your domain name. You may also change the $TTL.
			$ORIGIN example.com.
			$TTL    3600
			test.com    	IN	CNAME	link.storjshare.io.
			txt-test.com	IN	TXT  	storj-root:some/prefix
			txt-test.com	IN	TXT  	storj-access:accesskeyid
		`

		state.Succeed(t, "share", "--access", access, "--not-after=none", "--dns", "test.com", "--auth-service", authAddr, "sj://some/prefix").RequireStdoutGlob(t, expected)

		expected += "\ntxt-test.com	IN	TXT  	storj-tls:true\n"

		state.Succeed(t, "share", "--access", access, "--not-after=none", "--dns", "test.com", "--tls", "--auth-service", authAddr, "sj://some/prefix").RequireStdoutGlob(t, expected)
	})

	t.Run("register access and get restricted credentials", func(t *testing.T) {
		state := ultest.Setup(uplinkcli.Commands)

		mockServer.config.useFreeTierRestrictedExpiration = true

		expected := `
			Sharing access to satellite *
			=========== ACCESS RESTRICTIONS ==========================================================
			Download     : Allowed
			Upload       : Disallowed
			Lists        : Allowed
			Deletes      : Disallowed
			NotBefore    : No restriction
			NotAfter     : No restriction
			MaxObjectTTL : Not set
			Paths        : sj://some/prefix
			=========== SERIALIZED ACCESS WITH THE ABOVE RESTRICTIONS TO SHARE WITH OTHERS ===========
			Access       : *
			========== GATEWAY CREDENTIALS ===========================================================
			Trial account credentials automatically expire.
			Expiration   : 2006-01-02 15:04:05
			Access Key ID: accesskeyid
			Secret Key   : secretkey
			Endpoint     : endpoint
			Public Access: false
		`

		state.Succeed(t, "share", "--access", access, "--register", "--auth-service", authAddr, "sj://some/prefix").RequireStdoutGlob(t, expected)
	})
}

type drpcAuthMockServer struct {
	pb.DRPCEdgeAuthServer

	config drpcAuthMockServerConfig

	listener net.Listener
}

type drpcAuthMockServerConfig struct {
	useFreeTierRestrictedExpiration bool
}

func startDRPCAuthMockServer(ctx context.Context, t *testing.T, config drpcAuthMockServerConfig) *drpcAuthMockServer {
	server := &drpcAuthMockServer{
		config: config,
	}

	mux := drpcmux.New()
	err := pb.DRPCRegisterEdgeAuth(mux, server)
	require.NoError(t, err)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server.listener = listener

	go func() {
		require.NoError(t, drpcserver.New(mux).Serve(ctx, listener))
	}()

	return server
}

func (server *drpcAuthMockServer) addr() string {
	return "insecure://" + server.listener.Addr().String()
}

func (server *drpcAuthMockServer) RegisterAccess(context.Context, *pb.EdgeRegisterAccessRequest) (*pb.EdgeRegisterAccessResponse, error) {
	var expiration *time.Time
	if server.config.useFreeTierRestrictedExpiration {
		t := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
		expiration = &t
	}
	return &pb.EdgeRegisterAccessResponse{
		AccessKeyId:                  "accesskeyid",
		SecretKey:                    "secretkey",
		Endpoint:                     "endpoint",
		FreeTierRestrictedExpiration: expiration,
	}, nil
}
