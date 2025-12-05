// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"context"
	"testing"

	"github.com/zeebo/errs"

	"storj.io/common/testcontext"
	uplinkcli "storj.io/storj/cmd/uplink"
	"storj.io/storj/cmd/uplink/ultest"
)

func TestAccessSetup_FreeTierCredentialsExpiration(t *testing.T) {
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

	state.Succeed(t, "access", "setup", "--auth-service", authAddr).RequireStdout(t, `
		Imported access "my-access" to ""
		Switched default access to "my-access"
		========== GATEWAY CREDENTIALS ===========================================================
		Trial account credentials automatically expire.
		Expiration   : 2006-01-02 15:04:05
		Access Key ID: accesskeyid
		Secret Key   : secretkey
		Endpoint     : endpoint
	`)
}

type accessSetupPromptResponderOpts struct {
	accessName  string
	accessGrant string
	register    bool
}

func newAccessSetupPromptResponder(opts accessSetupPromptResponderOpts) ultest.PromptResponder {
	registerStr := "y"
	if !opts.register {
		registerStr = "n"
	}

	responses := map[string]string{
		"Enter name to import as [default: main]:":                           opts.accessName,
		"Enter API key or Access grant:":                                     opts.accessGrant,
		"Would you like S3 backwards-compatible Gateway credentials? (y/N):": registerStr,
	}

	return func(ctx context.Context, prompt string) (response string, err error) {
		response, ok := responses[prompt]
		if !ok {
			return "", errs.New("unknown prompt %q", prompt)
		}
		return response, nil
	}
}
