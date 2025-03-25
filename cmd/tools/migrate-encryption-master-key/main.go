// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	goFlag "flag"

	"github.com/jackc/pgx/v5"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/storj/satellite/kms"
)

var mon = monkit.Package()

var (
	rootCmd = &cobra.Command{
		Use:   "migrate-encryption-master-key",
		Short: "migrate-encryption-master-key",
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "run migrate-encryption-master-key",
		RunE:  run,
	}

	config Config
)

func init() {
	rootCmd.AddCommand(runCmd)

	config.BindFlags(runCmd.Flags())
}

// Config defines configuration for migration.
type Config struct {
	SatelliteDB string
	Limit       int

	Provider   string
	OldKeyInfo kms.KeyInfos
	NewKeyInfo kms.KeyInfos

	// TestMockKmsClient is used to mock the kms client for testing.
	TestMockKmsClient bool
}

// NewKeyID returns the new key ID.
func (config *Config) NewKeyID() int {
	for i := range config.NewKeyInfo.Values {
		return i
	}
	return 0
}

// OldKeyID returns the old key ID.
func (config *Config) OldKeyID() int {
	for i := range config.OldKeyInfo.Values {
		return i
	}
	return 0
}

// AllKeyInfos returns all key infos.
func (config *Config) AllKeyInfos() kms.KeyInfos {
	infos := make(map[int]kms.KeyInfo)
	for i, info := range config.OldKeyInfo.Values {
		infos[i] = info
	}
	for i, info := range config.NewKeyInfo.Values {
		infos[i] = info
	}

	return kms.KeyInfos{Values: infos}
}

// BindFlags adds bench flags to the flagset.
func (config *Config) BindFlags(flag *flag.FlagSet) {
	flag.StringVar(&config.SatelliteDB, "satellitedb", "", "connection URL for satelliteDB")
	flag.StringVar(&config.Provider, "provider", "gsm", "the provider of the passphrase encryption keys: 'gsm' for google, 'local' for local files")
	flag.AddGoFlag(&goFlag.Flag{
		Name:  "old-key-info",
		Usage: "the key from which to migrate projects, in the form key-id:version,checksum",
		Value: &config.OldKeyInfo,
	})
	flag.AddGoFlag(&goFlag.Flag{
		Name:  "new-key-info",
		Usage: "the key to which to migrate projects, in the form key-id:version,checksum",
		Value: &config.NewKeyInfo,
	})
	flag.IntVar(&config.Limit, "limit", 1000, "number of updates to perform at once")
}

// VerifyFlags verifies whether the values provided are valid.
func (config *Config) VerifyFlags() error {
	var errlist errs.Group
	if config.SatelliteDB == "" {
		errlist.Add(errors.New("flag '--satellitedb' is not set"))
	}
	if len(config.OldKeyInfo.Values) == 0 {
		errlist.Add(errors.New("flag '--old-key-info' is not set"))
	}
	if len(config.NewKeyInfo.Values) == 0 {
		errlist.Add(errors.New("flag '--new-key-info' is not set"))
	}
	if config.Provider == "" {
		errlist.Add(errors.New("flag '--provider' is not set"))
	}
	return errlist.Err()
}

func run(cmd *cobra.Command, args []string) error {
	if err := config.VerifyFlags(); err != nil {
		return err
	}

	ctx, _ := process.Ctx(cmd)
	log := zap.L()
	return Migrate(ctx, log, config)
}

func main() {
	logger, _, _ := process.NewLogger("migrate-encryption-master-key")
	zap.ReplaceGlobals(logger)

	process.Exec(rootCmd)
}

// Migrate updates projects with a new public_id where public_id is null.
func Migrate(ctx context.Context, log *zap.Logger, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := pgx.Connect(ctx, config.SatelliteDB)
	if err != nil {
		return errs.New("unable to connect %q: %w", config.SatelliteDB, err)
	}
	defer func() {
		err = errs.Combine(err, conn.Close(ctx))
	}()

	return MigrateEncryptionPassphrases(ctx, log, conn, config)
}
