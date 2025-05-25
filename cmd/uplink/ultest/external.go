// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ultest

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/uplink"
)

// TestAccess is a valid access grant intended for use in tests.
const TestAccess = "12edqrJX1V243n5fWtUrwpMQXL8gKdY2wbyqRPSG3rsA1tzmZiQjtCyF896egifN2C2qdY6g5S1t6e8iDhMUon9Pb7HdecBFheAcvmN8652mqu8hRx5zcTUaRTWfFCKS2S6DHmTeqPUHJLEp6cJGXNHcdqegcKfeahVZGP4rTagHvFGEraXjYRJ3knAcWDGW6BxACqogEWez6r274JiUBfs4yRSbRNRqUEURd28CwDXMSHLRKKA7TEDKEdQ"

// Ensure that external implements ulext.External.
var _ ulext.External = (*external)(nil)

type external struct {
	ulext.ExternalUnsupported

	fs              ulfs.Filesystem
	project         *uplink.Project
	promptResponder PromptResponder

	defaultAccessName string
	accesses          map[string]string
}

func newExternal(fs ulfs.Filesystem, project *uplink.Project, promptResponder PromptResponder) *external {
	return &external{
		fs:                fs,
		project:           project,
		promptResponder:   promptResponder,
		defaultAccessName: "TestAccessA",
		accesses: map[string]string{
			"TestAccessA": TestAccess,
			"TestAccessB": "1QiUjN497AySNH4ZX3wJCUZZNGKzpJwmZ1EcjKGgNR3Z9ADLawZNJbHXqm6VjH71nbWRRX6KfR9HHCr8sH3G9LA8e9qGuqWqkPPeskbD3Z12y4NuyxzwHYvcTSxa3Xk35Ts3ESGvP4785Rgeu5H8BF4kDriic6tRVUTPcAaYGCbHJPC2AfyPijLg4zZ627EuzeuWuo12mWGWiAZW3JJaVwD4657UJTGaUcuQqZxsjA1eTDkNFRfbv7zt9nW5si3E8FC6ZZFQ",
		},
	}
}

func (ex *external) OpenFilesystem(ctx context.Context, access string, options ...ulext.Option) (ulfs.Filesystem, error) {
	return ex.fs, nil
}

func (ex *external) OpenProject(ctx context.Context, access string, options ...ulext.Option) (*uplink.Project, error) {
	return ex.project, nil
}

func (ex *external) GetEdgeUrlOverrides(ctx context.Context, access *uplink.Access) (_ ulext.EdgeURLOverrides, err error) {
	return ulext.EdgeURLOverrides{}, nil
}

func (ex *external) PromptInput(ctx context.Context, prompt string) (string, error) {
	if ex.promptResponder == nil {
		return "", errs.New("no prompt responder configured")
	}
	return ex.promptResponder(ctx, prompt)
}

func (ex *external) PromptSecret(ctx context.Context, prompt string) (string, error) {
	return ex.PromptInput(ctx, prompt)
}

func (ex *external) OpenAccess(accessName string) (access *uplink.Access, err error) {
	accessDefault, accesses, err := ex.GetAccessInfo(true)
	if err != nil {
		return nil, err
	}
	if accessName != "" {
		accessDefault = accessName
	}
	if data, ok := accesses[accessDefault]; ok {
		access, err = uplink.ParseAccess(data)
	} else {
		access, err = uplink.ParseAccess(accessDefault)
		// TODO: if this errors then it's probably a name so don't report an error
		// that says "it failed to parse"
	}
	if err != nil {
		return nil, err
	}
	return access, nil
}

func (ex *external) AccessInfoFile() (string, error) {
	return "", nil
}

func (ex *external) GetAccessInfo(required bool) (string, map[string]string, error) {
	return ex.defaultAccessName, ex.accesses, nil
}

func (ex *external) SaveAccessInfo(defaultName string, accesses map[string]string) error {
	ex.defaultAccessName = defaultName
	ex.accesses = accesses
	return nil
}
