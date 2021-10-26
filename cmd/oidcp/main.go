package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dexidp/dex/connector"
	dexlog "github.com/dexidp/dex/pkg/log"
	"github.com/dexidp/dex/server"
	"github.com/dexidp/dex/storage"
	"github.com/dexidp/dex/storage/sql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"
	"storj.io/common/lrucache"

	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/satellitedb"
)

type Connector struct {
	authDB console.AuthDB
}

func (c *Connector) Prompt() string {
	return ""
}

func (c *Connector) Login(ctx context.Context, s connector.Scopes, email, password string) (identity connector.Identity, validPassword bool, err error) {
	user, err := c.authDB.Users().GetByEmail(ctx, email)
	if err != nil {
		return identity, false, err
	}

	err = bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(password))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return identity, false, nil
	} else if err != nil {
		return identity, false, err
	}

	identity = connector.Identity{
		UserID:        user.ID.String(),
		Username:      user.Email,
		Email:         user.Email,
		Groups:        []string{},
	}

	return identity, true, nil
}

func (c *Connector) Open(id string, logger dexlog.Logger) (connector.Connector, error) {
	return c, nil
}

var _ server.ConnectorConfig = &Connector{}
var _ connector.PasswordConnector = &Connector{}

type StorageConfig struct {
	SQLite   *sql.SQLite3  `json:"sqlite"`
	Postgres *sql.Postgres `json:"postgres"`
}

type Config struct {
	Storage *StorageConfig `json:"storage"`

	Database string `json:"database" help:"satellite database connection string" releaseDefault:"postgres://" devDefault:"postgres://"`

	DatabaseOptions struct {
		APIKeysCache struct {
			Expiration time.Duration `help:"satellite database api key expiration" default:"60s"`
			Capacity   int           `help:"satellite database api key lru capacity" default:"1000"`
		}
		RevocationsCache struct {
			Expiration time.Duration `help:"macaroon revocation cache expiration" default:"5m"`
			Capacity   int           `help:"macaroon revocation cache capacity" default:"10000"`
		}
	}
}

func main() {
	log := zap.L()

	cfg := &Config{
		Storage: &StorageConfig{
			SQLite: &sql.SQLite3{
				File: ":memory:",
			},
			// Postgres: &sql.Postgres{},
		},
	}

	cmd := cobra.Command{
		Use:   "oidcp",
		Short: "Runs the storj OIDC provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			db, err := satellitedb.Open(ctx, log.Named("db"), cfg.Database, satellitedb.Options{
				ApplicationName: "satellite-api",
				APIKeysLRUOptions: lrucache.Options{
					Expiration: cfg.DatabaseOptions.APIKeysCache.Expiration,
					Capacity:   cfg.DatabaseOptions.APIKeysCache.Capacity,
				},
				RevocationLRUOptions: lrucache.Options{
					Expiration: cfg.DatabaseOptions.RevocationsCache.Expiration,
					Capacity:   cfg.DatabaseOptions.RevocationsCache.Capacity,
				},
			})
			if err != nil {
				return err
			}

			server.ConnectorsConfig["storj"] = func() server.ConnectorConfig {
				return &Connector{
					authDB: db.Console(),
				}
			}

			dexLog := logrus.New()
			var store storage.Storage
			switch {
			case cfg.Storage.SQLite != nil:
				store, err = cfg.Storage.SQLite.Open(dexLog)
			case cfg.Storage.Postgres != nil:
				store, err = cfg.Storage.Postgres.Open(dexLog)
			default:
				err = fmt.Errorf("storage not specified")
			}

			if err != nil {
				return err
			}

			store = storage.WithStaticClients(store, []storage.Client{
				{
					// read client id and client secret from environment variables
					IDEnv:     "STORJ_CLIENT_ID",
					SecretEnv: "STORJ_CLIENT_SECRET",
					RedirectURIs: []string{
						"https://us1.storj.io",
					},
					Public:  true,
					Name:    "Storj",
					LogoURL: "",
				},
			})

			store = storage.WithStaticConnectors(store, []storage.Connector{
				{
					ID:     "storj",
					Type:   "storj",
					Name:   "Storj",
					Config: []byte{},
				},
			})

			svr, err := server.NewServer(ctx, server.Config{
				Issuer:                 "http://localhost:9998/",
				Storage:                store,
				SupportedResponseTypes: []string{},
				AllowedOrigins:         []string{},
				PasswordConnector:      "storj",
				// RotateKeysAfter: ,
				// IDTokensValidFor: ,
				// Now: ,
				// Web: ,
				Logger: dexLog,
				// PrometheusRegistry: ,
			})

			if err != nil {
				return err
			}

			group, ctx := errgroup.WithContext(ctx)
			group.Go(func() error {
				log.Info("starting http server on :9998")
				return http.ListenAndServe(":9998", svr)
			})

			return group.Wait()
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	halt := make(chan os.Signal, 1)
	signal.Notify(halt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-halt
		signal.Stop(halt)
		cancel()
	}()

	if err := cmd.ExecuteContext(ctx); err != nil {
		log.Fatal(err.Error())
	}
}
