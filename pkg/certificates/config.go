// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"context"
	"os"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// CertClientConfig is a config struct for use with a certificate signing service client
type CertClientConfig struct {
	Address string `help:"address of the certificate signing rpc service"`
}

// CertServerConfig is a config struct for use with a certificate signing service server
type CertServerConfig struct {
	Overwrite          bool   `default:"false" help:"if true, overwrites config AND authorization db is truncated"`
	AuthorizationDBURL string `default:"bolt://$CONFDIR/authorizations.db" help:"url to the certificate signing authorization database"`
	MinDifficulty      uint   `default:"30" help:"minimum difficulty of the requester's identity required to claim an authorization"`
	CA                 identity.FullCAConfig
}

// Sign submits a certificate signing request given the config
func (c CertClientConfig) Sign(ctx context.Context, ident *identity.FullIdentity, authToken string) ([][]byte, error) {
	client, err := NewClient(ctx, ident, c.Address)
	if err != nil {
		return nil, err
	}

	return client.Sign(ctx, authToken)
}

// NewAuthDB creates or opens the authorization database specified by the config
func (c CertServerConfig) NewAuthDB() (*AuthorizationDB, error) {
	// TODO: refactor db selection logic?
	driver, source, err := utils.SplitDBURL(c.AuthorizationDBURL)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	authDB := new(AuthorizationDB)
	switch driver {
	case "bolt":
		_, err := os.Stat(source)
		if c.Overwrite && err == nil {
			if err := os.Remove(source); err != nil {
				return nil, err
			}
		}

		authDB.DB, err = boltdb.New(source, AuthorizationsBucket)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}
	case "redis":
		redisClient, err := redis.NewClientFrom(c.AuthorizationDBURL)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}

		if c.Overwrite {
			if err := redisClient.FlushDB(); err != nil {
				return nil, err
			}
		}

		authDB.DB = redisClient
	default:
		return nil, ErrAuthorizationDB.New("database scheme not supported: %s", driver)
	}

	return authDB, nil
}

// Run implements the responsibility interface, starting a certificate signing server.
func (c CertServerConfig) Run(ctx context.Context, server *server.Server) (err error) {
	defer mon.Task()(&ctx)(&err)

	authDB, err := c.NewAuthDB()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, authDB.Close())
	}()

	signer, err := c.CA.Load()
	if err != nil {
		return err
	}

	srv := NewServer(
		zap.L(),
		signer,
		authDB,
		uint16(c.MinDifficulty),
	)
	pb.RegisterCertificatesServer(server.GRPC(), srv)

	srv.log.Info(
		"Certificate signing server running",
		zap.String("address", server.Addr().String()),
	)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		defer cancel()
		<-ctx.Done()
		return server.Close()
	})
	group.Go(func() error {
		defer cancel()
		return server.Run(ctx)
	})

	return group.Wait()
}
