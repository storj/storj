// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"io"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/private/server"
	"storj.io/storj/storagenode/internalpb"
)

// forgetSatelliteCfg defines configuration for forget-satellite command.
type forgetSatelliteCfg struct {
	Identity identity.Config
	Server   server.Config

	InitFSOptions
}

// InitFSOptions defines options for forget-satellite command.
type InitFSOptions struct {
	SatelliteIDs []string `internal:"true"`

	AllUntrusted bool `help:"Clean up all untrusted satellites" default:"false"`
	Force        bool `help:"Force removal of satellite data if not listed in satelliteDB cache or marked as untrusted" default:"false"`

	Stdout io.Writer `internal:"true"`
}

func newForgetSatelliteCmd(f *Factory) *cobra.Command {
	var cfg forgetSatelliteCfg
	cmd := &cobra.Command{
		Use:   "forget-satellite [satellite_IDs...]",
		Short: "Initiate forget satellite",
		Long:  "The command sends a request to the storagenode to initiate cleanup of untrusted satellite data.\n",
		Example: `
# Specify satellite ID to forget
$ storagenode forget-satellite --identity-dir /path/to/identityDir --config-dir /path/to/configDir satellite_ID

# Specify multiple satellite IDs to forget
$ storagenode forget-satellite satellite_ID1 satellite_ID2 --identity-dir /path/to/identityDir --config-dir /path/to/configDir

# Clean up all untrusted satellites
# This checks for untrusted satellites in both the satelliteDB cache and the excluded satellites list
# specified in the config.yaml file
$ storagenode forget-satellite --all-untrusted --identity-dir /path/to/identityDir --config-dir /path/to/configDir

# For force removal of data for untrusted satellites that are not listed in satelliteDB cache or marked as untrusted
$ storagenode forget-satellite satellite_ID1 satellite_ID2 --force --identity-dir /path/to/identityDir --config-dir /path/to/configDir
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.SatelliteIDs = args
			if len(args) > 0 && cfg.AllUntrusted {
				return errs.New("cannot specify both satellite IDs and --all-untrusted")
			}

			if len(args) == 0 && !cfg.AllUntrusted {
				return errs.New("must specify either satellite ID(s) as arguments or --all-untrusted flag")
			}

			if cfg.AllUntrusted && cfg.Force {
				return errs.New("cannot specify both --all-untrusted and --force")
			}

			ctx, _ := process.Ctx(cmd)
			return cmdForgetSatellite(ctx, zap.L(), &cfg)
		},
		Annotations: map[string]string{"type": "helper"},
	}

	process.Bind(cmd, &cfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func cmdForgetSatellite(ctx context.Context, log *zap.Logger, cfg *forgetSatelliteCfg) (err error) {
	if cfg.Stdout == nil {
		cfg.Stdout = os.Stdout
	}
	// we don't really need the identity, but we load it as a sanity check
	ident, err := cfg.Identity.Load()
	if err != nil {
		log.Fatal("Failed to load identity.", zap.Error(err))
	} else {
		log.Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	client, err := dialForgetSatelliteClient(ctx, cfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, client.close())
		if err != nil {
			log.Debug("error closing forget-satellite client", zap.Error(err))
		}
	}()

	return initForgetSatellite(ctx, log, client, cfg.InitFSOptions)
}

func initForgetSatellite(ctx context.Context, log *zap.Logger, client *forgetSatelliteClient, cfg InitFSOptions) (err error) {
	if cfg.AllUntrusted {
		resp, err := client.getUntrustedSatellites(ctx)
		if err != nil {
			return errs.Wrap(err)
		}

		if len(resp.SatelliteIds) == 0 {
			log.Info("No untrusted satellites found. You can add satellites to the exclusions list in the config.yaml file.")
			return nil
		}

		for _, satelliteID := range resp.SatelliteIds {
			cfg.SatelliteIDs = append(cfg.SatelliteIDs, satelliteID.String())
		}
	}

	statuses := make([]*forgetSatelliteStatus, 0, len(cfg.SatelliteIDs))
	for _, satelliteID := range cfg.SatelliteIDs {
		id, err := storj.NodeIDFromString(satelliteID)
		if err != nil {
			return errs.Wrap(err)
		}

		resp, err := client.initForgetSatellite(ctx, id, cfg.Force)
		if err != nil {
			inProgress := false
			switch rpcstatus.Code(err) {
			case rpcstatus.NotFound:
				log.Error("Satellite not found. Specify --force to force data deletion", zap.Stringer("Satellite ID", id))
			case rpcstatus.AlreadyExists:
				log.Error("Satellite is already being cleaned up", zap.Stringer("Satellite ID", id))
				inProgress = true
			case rpcstatus.FailedPrecondition:
				log.Error("Satellite is not untrusted. Specify --force to force data deletion", zap.Stringer("Satellite ID", id))
			default:
				log.Error("Failed to initialize forget satellite", zap.Stringer("Satellite ID", id), zap.Error(err))
			}
			statuses = append(statuses, &forgetSatelliteStatus{satelliteID: id, inProgress: inProgress, successful: false})
			continue
		}

		statuses = append(statuses, &forgetSatelliteStatus{satelliteID: id, inProgress: resp.InProgress, successful: false})
	}

	w := tabwriter.NewWriter(cfg.Stdout, 0, 0, 2, ' ', 0)
	return displayStatus(w, statuses)
}

type forgetSatelliteStatus struct {
	satelliteID storj.NodeID
	inProgress  bool
	successful  bool
}

func displayStatus(w *tabwriter.Writer, statuses []*forgetSatelliteStatus) (err error) {
	defer func() { err = errs.Combine(err, w.Flush()) }()

	_, err = w.Write([]byte("Satellite ID\tStatus\n"))
	if err != nil {
		return errs.Wrap(err)
	}

	for _, status := range statuses {
		_, err = w.Write([]byte(status.satelliteID.String() + "\t"))
		if err != nil {
			return errs.Wrap(err)
		}

		if status.inProgress {
			_, err = w.Write([]byte("In Progress\n"))
			if err != nil {
				return errs.Wrap(err)
			}
		} else if status.successful {
			_, err = w.Write([]byte("Successful\n"))
			if err != nil {
				return errs.Wrap(err)
			}
		} else {
			_, err = w.Write([]byte("Failed\n"))
			if err != nil {
				return errs.Wrap(err)
			}
		}
	}

	return err
}

type forgetSatelliteClient struct {
	conn *rpc.Conn
}

func dialForgetSatelliteClient(ctx context.Context, address string) (*forgetSatelliteClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &forgetSatelliteClient{conn: conn}, nil
}

func (client *forgetSatelliteClient) getUntrustedSatellites(ctx context.Context) (*internalpb.GetUntrustedSatellitesResponse, error) {
	return internalpb.NewDRPCNodeForgetSatelliteClient(client.conn).GetUntrustedSatellites(ctx, &internalpb.GetUntrustedSatellitesRequest{})

}

func (client *forgetSatelliteClient) initForgetSatellite(ctx context.Context, id storj.NodeID, force bool) (*internalpb.InitForgetSatelliteResponse, error) {
	return internalpb.NewDRPCNodeForgetSatelliteClient(client.conn).InitForgetSatellite(ctx, &internalpb.InitForgetSatelliteRequest{SatelliteId: id, ForceCleanup: force})
}

func (client *forgetSatelliteClient) close() error {
	return client.conn.Close()
}
