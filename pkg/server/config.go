// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"net"

	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/utils"
)

// Config holds server specific configuration parameters
type Config struct {
	PublicAddress  string `help:"public address to listen on" default:":7777"`
	PrivateAddress string `help:"private address to listen on" default:"localhost:0"`

	Identity         identity.Config
	CertVerification CertVerificationConfig
}

// Run will run the given responsibilities with the configured identity.
func (sc Config) Run(ctx context.Context, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	ident, err := sc.Identity.Load()
	if err != nil {
		return err
	}

	pcvs, revdb, err := sc.CertVerification.Load()
	if err != nil {
		return err
	}
	defer func() { err = utils.CombineErrors(err, revdb.Close()) }()

	publicLis, err := net.Listen("tcp", sc.PublicAddress)
	if err != nil {
		return err
	}
	defer func() { _ = publicLis.Close() }()

	privateLis, err := net.Listen("tcp", sc.PrivateAddress)
	if err != nil {
		return err
	}
	defer func() { _ = privateLis.Close() }()

	publicSrv, privateSrv, err := SetupRPCs(zap.L(), ident, pcvs)
	if err != nil {
		return err
	}

	s := NewServer(ident,
		NewHandle(publicSrv, publicLis),
		NewHandle(privateSrv, privateLis),
		services...)
	defer func() { err = utils.CombineErrors(err, s.Close()) }()

	zap.S().Infof("Node %s started", s.Identity().ID)
	return s.Run(ctx)
}
