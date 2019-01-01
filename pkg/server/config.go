// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"context"
	"crypto/x509"
	"io/ioutil"
	"net"
	"os"

	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/utils"
)

// ServerConfig holds server specific configuration parameters
type ServerConfig struct {
	RevocationDBURL     string `help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)" default:"bolt://$CONFDIR/revocations.db"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified)"`
	Address             string `help:"address to listen on" default:":7777"`
	Extensions          peertls.TLSExtConfig

	Identity identity.IdentityConfig
}

// Run will run the given responsibilities with the configured identity.
func (sc ServerConfig) Run(ctx context.Context,
	interceptor grpc.UnaryServerInterceptor, services ...Service) (err error) {
	defer mon.Task()(&ctx)(&err)

	ident, err := sc.Identity.Load()
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", sc.Address)
	if err != nil {
		return err
	}
	defer func() { _ = lis.Close() }()

	opts, err := NewServerOptions(ident, sc)
	if err != nil {
		return err
	}
	defer func() { err = utils.CombineErrors(err, opts.RevDB.Close()) }()

	s, err := NewServer(opts, lis, interceptor, services...)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	zap.S().Infof("Node %s started", s.Identity().ID)
	return s.Run(ctx)
}

// NewRevDB returns a new revocation database given the config
func (c ServerConfig) NewRevDB() (*peertls.RevocationDB, error) {
	driver, source, err := utils.SplitDBURL(c.RevocationDBURL)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	var db *peertls.RevocationDB
	switch driver {
	case "bolt":
		db, err = peertls.NewRevocationDBBolt(source)
		if err != nil {
			return nil, peertls.ErrRevocationDB.Wrap(err)
		}
		zap.S().Info("Starting overlay cache with BoltDB")
	case "redis":
		db, err = peertls.NewRevocationDBRedis(c.RevocationDBURL)
		if err != nil {
			return nil, peertls.ErrRevocationDB.Wrap(err)
		}
		zap.S().Info("Starting overlay cache with Redis")
	default:
		return nil, peertls.ErrRevocationDB.New("database scheme not supported: %s", driver)
	}

	return db, nil
}

// configure adds peer certificate verification functions and revocation
// database to the config.
func (c ServerConfig) configure(opts *ServerOptions) (err error) {
	var pcvs []peertls.PeerCertVerificationFunc
	parseOpts := peertls.ParseExtOptions{}

	if c.PeerCAWhitelistPath != "" {
		caWhitelist, err := loadWhitelist(c.PeerCAWhitelistPath)
		if err != nil {
			return err
		}
		parseOpts.CAWhitelist = caWhitelist
		pcvs = append(pcvs, peertls.VerifyCAWhitelist(caWhitelist))
	}

	if c.Extensions.Revocation {
		opts.RevDB, err = c.NewRevDB()
		if err != nil {
			return err
		}
		pcvs = append(pcvs, peertls.VerifyUnrevokedChainFunc(opts.RevDB))
	}

	exts := peertls.ParseExtensions(c.Extensions, parseOpts)
	pcvs = append(pcvs, exts.VerifyFunc())

	// NB: remove nil elements
	for i, f := range pcvs {
		if f == nil {
			copy(pcvs[i:], pcvs[i+1:])
			pcvs = pcvs[:len(pcvs)-1]
		}
	}

	opts.PCVFuncs = pcvs
	return nil
}

func loadWhitelist(path string) ([]*x509.Certificate, error) {
	w, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	var whitelist []*x509.Certificate
	if w != nil {
		whitelist, err = identity.DecodeAndParseChainPEM(w)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	}
	return whitelist, nil
}
