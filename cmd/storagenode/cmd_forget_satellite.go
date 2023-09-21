// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/private/cfgstruct"
	"storj.io/private/process"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/satellites"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/trust"
)

// runCfg defines configuration for run command.
type forgetSatelliteCfg struct {
	storagenode.Config

	SatelliteIDs []string `internal:"true"`

	AllUntrusted bool `help:"Clean up all untrusted satellites" default:"false"`
	Force        bool `help:"Force removal of satellite data if not listed in satelliteDB cache or marked as untrusted" default:"false"`
}

func newForgetSatelliteCmd(f *Factory) *cobra.Command {
	var cfg forgetSatelliteCfg
	cmd := &cobra.Command{
		Use:   "forget-satellite [satellite_IDs...]",
		Short: "Remove an untrusted satellite from the trust cache and clean up its data",
		Long: "Forget a satellite.\n" +
			"The command shows the list of the available untrusted satellites " +
			"and removes the selected satellites from the trust cache and clean up the available data",
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
	// we don't really need the identity, but we load it as a sanity check
	ident, err := cfg.Identity.Load()
	if err != nil {
		log.Fatal("Failed to load identity.", zap.Error(err))
	} else {
		log.Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	db, err := storagenodedb.OpenExisting(ctx, log.Named("db"), cfg.DatabaseConfig())
	if err != nil {
		return errs.New("Error starting master database on storagenode: %+v", err)
	}

	satelliteDB := db.Satellites()

	// get list of excluded satellites
	excludedSatellites := make(map[storj.NodeID]bool)
	for _, rule := range cfg.Storage2.Trust.Exclusions.Rules {
		url, err := trust.ParseSatelliteURL(rule.String())
		if err != nil {
			log.Warn("Failed to parse satellite URL from exclusions list", zap.Error(err), zap.String("rule", rule.String()))
			continue
		}
		excludedSatellites[url.ID] = false // false means the satellite has not been cleaned up yet.
	}

	if len(cfg.SatelliteIDs) > 0 {
		for _, satelliteIDStr := range cfg.SatelliteIDs {
			satelliteID, err := storj.NodeIDFromString(satelliteIDStr)
			if err != nil {
				return err
			}

			satellite := satellites.Satellite{
				SatelliteID: satelliteID,
				Status:      satellites.Untrusted,
			}

			// check if satellite is excluded
			cleanedUp, isExcluded := excludedSatellites[satelliteID]
			if !isExcluded {
				sat, err := satelliteDB.GetSatellite(ctx, satelliteID)
				if err != nil {
					return err
				}
				if !satellite.SatelliteID.IsZero() {
					satellite = sat
				}
				if satellite.SatelliteID.IsZero() && !cfg.Force {
					return errs.New("satellite %v not found. Specify --force to force data deletion", satelliteID)
				}
				log.Warn("Satellite not found in satelliteDB cache. Forcing removal of satellite data.", zap.Stringer("satelliteID", satelliteID))
			}

			if cleanedUp {
				log.Warn("Satellite already cleaned up", zap.Stringer("satelliteID", satelliteID))
				continue
			}

			err = cleanupSatellite(ctx, log, cfg, db, satellite)
			if err != nil {
				return err
			}
		}
	} else {
		sats, err := satelliteDB.GetSatellites(ctx)
		if err != nil {
			return err
		}

		hasUntrusted := false
		for _, satellite := range sats {
			if satellite.Status != satellites.Untrusted {
				continue
			}
			hasUntrusted = true
			err = cleanupSatellite(ctx, log, cfg, db, satellite)
			if err != nil {
				return err
			}
			excludedSatellites[satellite.SatelliteID] = true // true means the satellite has been cleaned up.
		}

		// clean up excluded satellites that might not be in the satelliteDB cache.
		for satelliteID, cleanedUp := range excludedSatellites {
			if !cleanedUp {
				satellite := satellites.Satellite{
					SatelliteID: satelliteID,
					Status:      satellites.Untrusted,
				}
				hasUntrusted = true
				err = cleanupSatellite(ctx, log, cfg, db, satellite)
				if err != nil {
					return err
				}
			}
		}

		if !hasUntrusted {
			log.Info("No untrusted satellites found. You can add satellites to the exclusions list in the config.yaml file.")
		}
	}

	return nil
}

func cleanupSatellite(ctx context.Context, log *zap.Logger, cfg *forgetSatelliteCfg, db *storagenodedb.DB, satellite satellites.Satellite) error {
	if satellite.Status != satellites.Untrusted && !cfg.Force {
		log.Error("Satellite is not untrusted. Skipping", zap.Stringer("satelliteID", satellite.SatelliteID))
		return nil
	}

	log.Info("Removing satellite from trust cache.", zap.Stringer("satelliteID", satellite.SatelliteID))
	cache, err := trust.LoadCache(cfg.Storage2.Trust.CachePath)
	if err != nil {
		return err
	}

	deleted := cache.DeleteSatelliteEntry(satellite.SatelliteID)
	if deleted {
		if err := cache.Save(ctx); err != nil {
			return err
		}
		log.Info("Satellite removed from trust cache.", zap.Stringer("satelliteID", satellite.SatelliteID))
	}

	log.Info("Cleaning up satellite data.", zap.Stringer("satelliteID", satellite.SatelliteID))
	blobs := pieces.NewBlobsUsageCache(log.Named("blobscache"), db.Pieces())
	if err := blobs.DeleteNamespace(ctx, satellite.SatelliteID.Bytes()); err != nil {
		return err
	}

	log.Info("Cleaning up the trash.", zap.Stringer("satelliteID", satellite.SatelliteID))
	err = blobs.DeleteTrashNamespace(ctx, satellite.SatelliteID.Bytes())
	if err != nil {
		return err
	}

	log.Info("Removing satellite info from reputation DB.", zap.Stringer("satelliteID", satellite.SatelliteID))
	err = db.Reputation().Delete(ctx, satellite.SatelliteID)
	if err != nil {
		return err
	}

	// delete v0 pieces for the satellite, if any.
	log.Info("Removing satellite v0 pieces if any.", zap.Stringer("satelliteID", satellite.SatelliteID))
	err = db.V0PieceInfo().WalkSatelliteV0Pieces(ctx, db.Pieces(), satellite.SatelliteID, func(access pieces.StoredPieceAccess) error {
		return db.Pieces().Delete(ctx, access.BlobRef())
	})
	if err != nil {
		return err
	}

	log.Info("Removing satellite from satellites DB.", zap.Stringer("satelliteID", satellite.SatelliteID))
	err = db.Satellites().DeleteSatellite(ctx, satellite.SatelliteID)
	if err != nil {
		return err
	}

	return nil
}
