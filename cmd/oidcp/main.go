package main

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-oauth2/oauth2/v4/generates"
	"github.com/go-oauth2/oauth2/v4/manage"
	"github.com/go-oauth2/oauth2/v4/server"
	"github.com/go-session/session"
	"github.com/jackc/pgx/v4"
	"github.com/spf13/cobra"
	pg "github.com/vgarvardt/go-oauth2-pg/v4"
	"github.com/vgarvardt/go-pg-adapter/pgx4adapter"
	"go.uber.org/zap"

	"storj.io/storj/cmd/oidcp/internal"
)

// go:embed templates
var templates *embed.FS

type Config struct {
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

	cfg := &Config{}

	cmd := cobra.Command{
		Use:   "oidcp",
		Short: "Runs the storj OIDC provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()

			//db, err := satellitedb.Open(ctx, log.Named("db"), cfg.Database, satellitedb.Options{
			//	ApplicationName: "satellite-api",
			//	APIKeysLRUOptions: lrucache.Options{
			//		Expiration: cfg.DatabaseOptions.APIKeysCache.Expiration,
			//		Capacity:   cfg.DatabaseOptions.APIKeysCache.Capacity,
			//	},
			//	RevocationLRUOptions: lrucache.Options{
			//		Expiration: cfg.DatabaseOptions.RevocationsCache.Expiration,
			//		Capacity:   cfg.DatabaseOptions.RevocationsCache.Capacity,
			//	},
			//})
			//if err != nil {
			//	return err
			//}

			manager := manage.NewDefaultManager()
			manager.SetAuthorizeCodeTokenCfg(manage.DefaultAuthorizeCodeTokenCfg)

			manager.MapAuthorizeGenerate(generates.NewAuthorizeGenerate()) // generate authorization_code
			manager.MapAccessGenerate(&internal.MacaroonAccessGenerate{})  // generate macaroon based access and refresh tokens

			// store everything in postgres
			conn, err := pgx.Connect(ctx, cfg.Database)
			if err != nil {
				return err
			}

			adapter := pgx4adapter.NewConn(conn)

			tokenStore, err := pg.NewTokenStore(adapter, pg.WithTokenStoreGCInterval(time.Minute))
			if err != nil {
				return err
			}
			manager.MapTokenStorage(tokenStore)
			manager.MustClientStorage(pg.NewClientStore(adapter))

			svr := server.NewDefaultServer(manager)

			svr.SetUserAuthorizationHandler(func(w http.ResponseWriter, r *http.Request) (userID string, err error) {
				store, err := session.Start(r.Context(), w, r)
				if err != nil {
					return
				}

				uid, ok := store.Get("LoggedInUserID")
				if !ok {
					if r.Form == nil {
						err = r.ParseForm()
						if err != nil {
							return
						}
					}

					store.Set("ReturnURI", r.Form)
					err = store.Save()
					if err != nil {
						return
					}

					w.Header().Set("Location", "/login")
					w.WriteHeader(http.StatusFound)
					return
				}

				store.Delete("LoggedInUserID")
				err = store.Save()
				if err != nil {
					return
				}

				userID = uid.(string)
				return
			})

			// render login, handle 2fa, etc
			// probably a port of logic from the storj console code
			http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {})

			// render consent screen
			http.HandleFunc("/consent", func(w http.ResponseWriter, r *http.Request) {})

			http.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) { // POST consent
				store, err := session.Start(r.Context(), w, r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}

				var form url.Values
				if v, ok := store.Get("ReturnURI"); ok {
					form = v.(url.Values)
				}
				r.Form = form

				store.Delete("ReturnURI")
				err = store.Save()
				if err != nil {
					return
				}

				err = svr.HandleAuthorizeRequest(w, r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
				}
			})

			http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
				err = svr.HandleTokenRequest(w, r)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			})

			// todo: implement OIDC functionality

			// probably don't need
			http.HandleFunc("/keys", func(w http.ResponseWriter, r *http.Request) {})

			http.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
				header := r.Header.Get("Authorization")
				if !strings.HasPrefix(header, "Bearer ") {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				apiKeyString := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))

				tokenInfo, err := tokenStore.GetByAccess(r.Context(), apiKeyString)
				if err != nil {
					http.Error(w, "bad request", http.StatusBadRequest)
					return
				}

				// tada! macaroon to userID mapping!
				data, _ := json.Marshal(map[string]interface{}{
					"user_id": tokenInfo.GetUserID(),
				})

				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			})

			http.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
				// provide well known configuration
			})

			return http.ListenAndServe(":9000", http.DefaultServeMux)
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
