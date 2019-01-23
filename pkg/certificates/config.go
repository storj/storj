// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"context"
	"os"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

// CertClientConfig is a config struct for use with a certificate signing service client
type CertClientConfig struct {
	AuthToken string `help:"authorization token to use to claim a certificate signing request (only applicable for the alpha network)"`
	Address   string `help:"address of the certificate signing rpc service"`
}

// CertServerConfig is a config struct for use with a certificate signing service server
type CertServerConfig struct {
	Overwrite          bool   `default:"false" help:"if true, overwrites config AND authorization db is truncated"`
	AuthorizationDBURL string `default:"bolt://$CONFDIR/authorizations.db" help:"url to the certificate signing authorization database"`
	MinDifficulty      uint   `default:"30" help:"minimum difficulty of the requester's identity required to claim an authorization"`
	CA                 identity.FullCAConfig
}

// SetupIdentity loads or creates a CA and identity and submits a certificate
// signing request request for the CA; if successful, updated chains are saved.
func (c CertClientConfig) SetupIdentity(
	ctx context.Context,
	caConfig identity.CASetupConfig,
	identConfig identity.SetupConfig,
) error {
	caStatus := caConfig.Status()
	var (
		ca    *identity.FullCertificateAuthority
		ident *identity.FullIdentity
		err   error
	)
	switch {
	case caStatus == identity.CertKey && !caConfig.Overwrite:
		ca, err = caConfig.FullConfig().Load()
		if err != nil {
			return err
		}
	case caStatus != identity.NoCertNoKey && !caConfig.Overwrite:
		return identity.ErrSetup.New("certificate authority file(s) exist: %s", caStatus)
	default:
		t, err := time.ParseDuration(caConfig.Timeout)
		if err != nil {
			return errs.Wrap(err)
		}
		ctx, cancel := context.WithTimeout(ctx, t)
		defer cancel()

		ca, err = caConfig.Create(ctx)
		if err != nil {
			return err
		}
	}

	identStatus := identConfig.Status()
	switch {
	case identStatus == identity.CertKey && !identConfig.Overwrite:
		ident, err = identConfig.FullConfig().Load()
		if err != nil {
			return err
		}
	case identStatus != identity.NoCertNoKey && !identConfig.Overwrite:
		return identity.ErrSetup.New("identity file(s) exist: %s", identStatus)
	default:
		ident, err = identConfig.Create(ca)
		if err != nil {
			return err
		}
	}

	signedChainBytes, err := c.Sign(ctx, ident)
	if err != nil {
		return errs.New("error occurred while signing certificate: %s\n(identity files were still generated and saved, if you try again existing files will be loaded)", err)
	}

	signedChain, err := identity.ParseCertChain(signedChainBytes)
	if err != nil {
		return nil
	}

	ca.Cert = signedChain[0]
	ca.RestChain = signedChain[1:]
	err = identity.FullCAConfig{
		CertPath: caConfig.FullConfig().CertPath,
	}.Save(ca)
	if err != nil {
		return err
	}

	ident.RestChain = signedChain[1:]
	err = identity.Config{
		CertPath: identConfig.FullConfig().CertPath,
	}.Save(ident)
	if err != nil {
		return err
	}
	return nil
}

// Sign submits a certificate signing request given the config
func (c CertClientConfig) Sign(ctx context.Context, ident *identity.FullIdentity) ([][]byte, error) {
	client, err := NewClient(ctx, ident, c.Address)
	if err != nil {
		return nil, err
	}

	return client.Sign(ctx, c.AuthToken)
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
func (c CertServerConfig) Run(ctx context.Context, server *provider.Provider) (err error) {
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

	go func() {
		done := ctx.Done()
		<-done
		if err := server.Close(); err != nil {
			srv.log.Error("closing server", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}
