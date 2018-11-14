package satelliteweb

import (
	"net/http"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/graphql-go/graphql"
)

// GatewayConfig contains configuration for gateway
type GatewayConfig struct {
	Address    string `help:"server address of the graphql api gateway and frontend app" default:"127.0.0.1:8081"`
	StaticPath string `help:"path to static resources" default:""`
}

type gateway struct {
	schema graphql.Schema
	config GatewayConfig
	logger *zap.SugaredLogger
}

func (gw *gateway) run() {
	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(gw.config.StaticPath))

	mux.Handle("/api/graphql/v0", http.HandlerFunc(gw.grapqlHandler))

	if gw.config.StaticPath != "" {
		mux.Handle("/", http.HandlerFunc(gw.appHandler))
		mux.Handle("/static/", http.StripPrefix("/static", fs))
	}

	err := http.ListenAndServe(gw.config.Address, mux)
	gw.logger.Errorf("unexpected exit of satellite gateway server: ", err)
}

func (gw *gateway) appHandler(w http.ResponseWriter, req *http.Request) {
	http.ServeFile(w, req, filepath.Join(gw.config.StaticPath, "dist", "public", "index.html"))
}
