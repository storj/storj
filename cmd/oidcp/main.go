package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/dexidp/dex/connector"
	dexlog "github.com/dexidp/dex/pkg/log"
	"github.com/dexidp/dex/server"
	"github.com/dexidp/dex/storage"
	"github.com/dexidp/dex/storage/sql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

type Connector struct {
}

func (c *Connector) Prompt() string {
	return ""
}

func (c *Connector) Login(ctx context.Context, s connector.Scopes, username, password string) (identity connector.Identity, validPassword bool, err error) {
	// todo: plug into our existing user management apis...
	panic("implement me")
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
}

func main() {
	log := logrus.New()

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

			server.ConnectorsConfig["storj"] = func() server.ConnectorConfig {
				return &Connector{}
			}

			var store storage.Storage
			var err error

			switch {
			case cfg.Storage.SQLite != nil:
				store, err = cfg.Storage.SQLite.Open(log)
			case cfg.Storage.Postgres != nil:
				store, err = cfg.Storage.Postgres.Open(log)
			default:
				err = fmt.Errorf("storage not specified")
			}

			if err != nil {
				return err
			}

			// todo: embed and parse with json?
			store = storage.WithStaticClients(store, []storage.Client{
				// allow our cloud to use OAuth for authentication
				{
					ID: "console",
					// IDEnv: "",
					Secret: "",
					// SecretEnv: "",
					RedirectURIs: []string{
						"https://us1.storj.io",
						"https://eu1.storj.io",
						"https://ap1.storj.io",
					},
					TrustedPeers: []string{},
					Public:       true,
					Name:         "Storj",
					LogoURL:      "",
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
				Logger: log,
				// PrometheusRegistry: ,
			})

			if err != nil {
				return err
			}

			group, ctx := errgroup.WithContext(ctx)
			group.Go(func() error {
				log.Println("starting http server on :9998")
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
		log.Fatal(err)
	}
}
