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

	"storj.io/common/cfgstruct"
	"storj.io/common/identity"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/storj/private/server"
	"storj.io/storj/storagenode/internalpb"
)

// forgetSatelliteCfg defines configuration for forget-satellite command.
type forgetSatelliteCfg struct {
	Identity identity.Config
	Server   server.Config

	ForgetSatelliteOptions
}

// ForgetSatelliteOptions defines options for forget-satellite command.
type ForgetSatelliteOptions struct {
	SatelliteIDs []string `internal:"true"`

	AllUntrusted bool `help:"Clean up all untrusted satellites" default:"false"`
	Force        bool `help:"Force removal of satellite data if not listed in satelliteDB cache or marked as untrusted" default:"false"`

	Stdout io.Writer `internal:"true"`
}

type forgetSatelliteStatusCfg struct {
	Identity identity.Config
	Server   server.Config

	SatelliteIDs []string  `internal:"true"`
	Stdout       io.Writer `internal:"true"`
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

func newForgetSatelliteStatusCmd(f *Factory) *cobra.Command {
	var cfg forgetSatelliteStatusCfg
	cmd := &cobra.Command{
		Use:   "forget-satellite-status [satellite_IDs...]",
		Short: "Get forget satellite status",
		Long:  "The command returns the status of the forget-satellite process for a satellite.\n",
		Example: `
# Get status for all processes
$ storagenode forget-satellite-status --identity-dir /path/to/identityDir --config-dir /path/to/configDir

# Specify satellite ID to get status
$ storagenode forget-satellite-status --identity-dir /path/to/identityDir --config-dir /path/to/configDir satellite_ID

# Specify multiple satellite IDs to get status
$ storagenode forget-satellite-status satellite_ID1 satellite_ID2 --identity-dir /path/to/identityDir --config-dir /path/to/configDir
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg.SatelliteIDs = args

			ctx, _ := process.Ctx(cmd)
			return cmdForgetSatelliteStatus(ctx, zap.L(), &cfg)
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

	return startForgetSatellite(ctx, log, client, cfg.ForgetSatelliteOptions)
}

func startForgetSatellite(ctx context.Context, log *zap.Logger, client *forgetSatelliteClient, opts ForgetSatelliteOptions) (err error) {
	if opts.AllUntrusted {
		resp, err := client.getUntrustedSatellites(ctx)
		if err != nil {
			return errs.Wrap(err)
		}

		if len(resp.SatelliteIds) == 0 {
			log.Info("No untrusted satellites found. You can add satellites to the exclusions list in the config.yaml file.")
			return nil
		}

		for _, satelliteID := range resp.SatelliteIds {
			opts.SatelliteIDs = append(opts.SatelliteIDs, satelliteID.String())
		}
	}

	statuses := make([]*forgetSatelliteStatus, 0, len(opts.SatelliteIDs))
	for _, satelliteID := range opts.SatelliteIDs {
		id, err := storj.NodeIDFromString(satelliteID)
		if err != nil {
			return errs.Wrap(err)
		}

		resp, err := client.startForgetSatellite(ctx, id, opts.Force)
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

	w := tabwriter.NewWriter(opts.Stdout, 0, 0, 2, ' ', 0)
	return displayStatus(w, statuses)
}

func cmdForgetSatelliteStatus(ctx context.Context, log *zap.Logger, cfg *forgetSatelliteStatusCfg) (err error) {
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

	statuses := make([]*forgetSatelliteStatus, 0, len(cfg.SatelliteIDs))
	for _, satelliteID := range cfg.SatelliteIDs {
		id, err := storj.NodeIDFromString(satelliteID)
		if err != nil {
			return errs.Wrap(err)
		}

		resp, err := client.getForgetSatelliteStatus(ctx, id)
		if err != nil {
			log.Error("Failed to get forget satellite status", zap.Stringer("Satellite ID", id), zap.Error(err))
			statuses = append(statuses, &forgetSatelliteStatus{satelliteID: id, inProgress: false, successful: false})
			continue
		}

		statuses = append(statuses, &forgetSatelliteStatus{satelliteID: id, inProgress: resp.InProgress, successful: resp.Successful})
	}

	if len(cfg.SatelliteIDs) == 0 {
		resp, err := client.getAllForgetSatelliteStatus(ctx)
		if err != nil {
			return errs.Wrap(err)
		}

		for _, status := range resp.Statuses {
			statuses = append(statuses, &forgetSatelliteStatus{satelliteID: status.SatelliteId, inProgress: status.InProgress, successful: status.Successful})
		}
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

func (client *forgetSatelliteClient) startForgetSatellite(ctx context.Context, id storj.NodeID, force bool) (*internalpb.InitForgetSatelliteResponse, error) {
	return internalpb.NewDRPCNodeForgetSatelliteClient(client.conn).InitForgetSatellite(ctx, &internalpb.InitForgetSatelliteRequest{SatelliteId: id, ForceCleanup: force})
}

func (client *forgetSatelliteClient) getForgetSatelliteStatus(ctx context.Context, id storj.NodeID) (*internalpb.ForgetSatelliteStatusResponse, error) {
	return internalpb.NewDRPCNodeForgetSatelliteClient(client.conn).ForgetSatelliteStatus(ctx, &internalpb.ForgetSatelliteStatusRequest{SatelliteId: id})
}

func (client *forgetSatelliteClient) getAllForgetSatelliteStatus(ctx context.Context) (*internalpb.GetAllForgetSatelliteStatusResponse, error) {
	return internalpb.NewDRPCNodeForgetSatelliteClient(client.conn).GetAllForgetSatelliteStatus(ctx, &internalpb.GetAllForgetSatelliteStatusRequest{})
}

func (client *forgetSatelliteClient) close() error {
	return client.conn.Close()
}
