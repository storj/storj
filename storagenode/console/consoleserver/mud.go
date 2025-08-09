// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleserver

import (
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
	"storj.io/storj/storagenode/console"
	"storj.io/storj/storagenode/notifications"
	"storj.io/storj/storagenode/payouts"
)

// Module registers the console server dependency injection components.
func Module(ball *mud.Ball, assets fs.FS) {
	config.RegisterConfig[Config](ball, "config")
	mud.Provide[*Server](ball, func(logger *zap.Logger, notifications *notifications.Service, service *console.Service, payout *payouts.Service, config Config) (*Server, error) {
		listener, err := net.Listen("tcp", config.Address)
		if err != nil {
			return nil, err
		}

		logger.Info("webui is started listening", zap.Stringer("addr", listener.Addr()))
		if config.StaticDir != "" {
			// HACKFIX: Previous setups specify the directory for web/storagenode,
			// instead of the actual built data. This is for backwards compatibility.
			distDir := filepath.Join(config.StaticDir, "dist")
			assets = os.DirFS(distDir)
		}
		return NewServer(logger, assets, notifications, service, payout, listener), nil
	})
	mud.Tag[*Server](ball, modular.Service{})
}
