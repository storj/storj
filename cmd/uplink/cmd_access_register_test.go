// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"testing"

	"storj.io/common/testcontext"
	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestAccessRegister_FreeTierCredentialsExpiration(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(testcontext.New(t))
	defer cancelCtx()

	mockServer := startDRPCAuthMockServer(ctx, t, drpcAuthMockServerConfig{useFreeTierRestrictedExpiration: true})
	authAddr := mockServer.addr()

	responder := newAccessSetupPromptResponder(accessSetupPromptResponderOpts{
		accessName:  "my-access",
		accessGrant: ultest.TestAccess,
		register:    true,
	})

	state := ultest.Setup(uplinkcli.Commands, ultest.WithPromptResponder(responder))

	t.Run("Default format", func(t *testing.T) {
		state.Succeed(t, "access", "register", "--auth-service", authAddr).RequireStdout(t, `
			========== GATEWAY CREDENTIALS ===========================================================
			Trial account credentials automatically expire.
			Expiration   : 2006-01-02 15:04:05
			Access Key ID: accesskeyid
			Secret Key   : secretkey
			Endpoint     : endpoint
		`)
	})

	t.Run("Environment variable format", func(t *testing.T) {
		state.Succeed(t, "access", "register", "--auth-service", authAddr, "--format", "env").RequireStdout(t, `
			# Your trial account credentials will expire at 2006-01-02 15:04:05.
			AWS_ACCESS_KEY_ID=accesskeyid
			AWS_SECRET_ACCESS_KEY=secretkey
			AWS_ENDPOINT=endpoint
		`)
	})

	t.Run("AWS configuration commands format", func(t *testing.T) {
		state.Succeed(t, "access", "register", "--auth-service", authAddr, "--format", "aws").RequireStdout(t, `
			# Your trial account credentials will expire at 2006-01-02 15:04:05.
			aws configure  set aws_access_key_id accesskeyid
			aws configure  set aws_secret_access_key secretkey
			aws configure  set s3.endpoint_url endpoint
		`)
	})

	t.Run("OM configuration commands format", func(t *testing.T) {
		state.Succeed(t, "access", "register", "--auth-service", authAddr, "--format", "om").RequireStdout(t, `
			aws_access_key_id = accesskeyid
			aws_secret_access_key = secretkey
			endpoint = endpoint
		`)
	})
}
