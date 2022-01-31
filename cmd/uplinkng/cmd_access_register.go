// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"

	"github.com/zeebo/clingy"

	"storj.io/storj/cmd/uplinkng/ulext"
)

type cmdAccessRegister struct {
	ex ulext.External

	accessNameOrValue *string
	authService       string
	public            bool
	format            string
	awsProfile        string
}

func newCmdAccessRegister(ex ulext.External) *cmdAccessRegister {
	return &cmdAccessRegister{ex: ex}
}

func (c *cmdAccessRegister) Setup(params clingy.Parameters) {
	c.authService = params.Flag("auth-service", "The address to the service you wish to register your access with", "https://auth.us1.storjshare.io").(string)
	c.public = params.Flag("public", "If true, the access will be public", false, clingy.Transform(strconv.ParseBool)).(bool)
	c.format = params.Flag("format", "Format of the output credentials, use 'env' or 'aws' when using in scripts", "").(string)
	c.awsProfile = params.Flag("aws-profile", "If using --format=aws, output the --profile tag using this profile", "").(string)

	c.accessNameOrValue = params.Arg("access", "The name or value of the access grant we're registering with the auth service", clingy.Optional).(*string)
}

func (c *cmdAccessRegister) Execute(ctx clingy.Context) (err error) {
	accessNameOrValue := ""
	if c.accessNameOrValue != nil && len(*c.accessNameOrValue) > 0 {
		accessNameOrValue = *c.accessNameOrValue
	}

	access, err := c.ex.OpenAccess(accessNameOrValue)
	if err != nil {
		return err
	}

	accessKey, secretKey, endpoint, err := RegisterAccess(ctx, access, c.authService, c.public, defaultAccessRegisterTimeout)
	if err != nil {
		return err
	}

	return DisplayGatewayCredentials(ctx, accessKey, secretKey, endpoint, c.format, c.awsProfile)
}
