// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"strconv"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplink/ulext"
)

type cmdAccessRegister struct {
	ex ulext.External

	accessNameOrValue *string
	authService       string
	caCert            string
	public            bool
	format            string
	awsProfile        string
}

func newCmdAccessRegister(ex ulext.External) *cmdAccessRegister {
	return &cmdAccessRegister{ex: ex}
}

func (c *cmdAccessRegister) Setup(params clingy.Parameters) {
	c.authService = params.Flag("auth-service", "The address to the service you wish to register your access with", "auth.storjshare.io:7777").(string)
	c.caCert = params.Flag("ca-cert", "path to a file in PEM format with certificate(s) or certificate chain(s) to validate the auth service against", "").(string)
	c.public = params.Flag("public", "If true, the access will be public", false, clingy.Transform(strconv.ParseBool)).(bool)
	c.format = params.Flag("format", "Format of the output credentials, use 'env', 'aws' or `om` (Object Mount) when using in scripts", "").(string)
	c.awsProfile = params.Flag("aws-profile", "If using --format=aws, output the --profile tag using this profile", "").(string)

	c.accessNameOrValue = params.Arg("access", "The name or value of the access grant we're registering with the auth service", clingy.Optional).(*string)
}

func (c *cmdAccessRegister) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	accessNameOrValue := ""
	if c.accessNameOrValue != nil && len(*c.accessNameOrValue) > 0 {
		accessNameOrValue = *c.accessNameOrValue
	}

	access, err := c.ex.OpenAccess(accessNameOrValue)
	if err != nil {
		return err
	}

	info, err := c.ex.GetEdgeUrlOverrides(ctx, access)
	if err != nil {
		return err
	}

	authService := c.authService

	if info.AuthService != "" {
		authService = info.AuthService
	}
	credentials, err := RegisterAccess(ctx, access, authService, c.public, c.caCert)
	if err != nil {
		return err
	}

	return DisplayGatewayCredentials(ctx, *credentials, c.format, c.awsProfile)
}
