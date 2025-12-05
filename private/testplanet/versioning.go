// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"storj.io/common/version"
	"storj.io/storj/private/version/checker"
	"storj.io/storj/versioncontrol"
)

// newVersionControlServer initializes the Versioning Server.
func (planet *Planet) newVersionControlServer() (peer *versioncontrol.Peer, err error) {
	prefix := "versioncontrol"
	log := planet.log.Named(prefix)
	dbDir := filepath.Join(planet.directory, prefix)

	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, err
	}

	var minimum, suggested versioncontrol.VersionConfig
	minimum.Version = "v0.0.1"
	suggested.Version = "v0.0.1"

	defaultProcessConfig := versioncontrol.ProcessConfig{
		Minimum:   minimum,
		Suggested: suggested,
		Rollout: versioncontrol.RolloutConfig{
			Seed: "0000000000000000000000000000000000000000000000000000000000000001",
		},
	}
	config := &versioncontrol.Config{
		Address: planet.NewListenAddress(),
		Versions: versioncontrol.OldVersionConfig{
			Satellite:   "v0.0.1",
			Storagenode: "v0.0.1",
			Uplink:      "v0.0.1",
			Gateway:     "v0.0.1",
			Identity:    "v0.0.1",
		},
		Binary: versioncontrol.ProcessesConfig{
			Satellite:          defaultProcessConfig,
			Storagenode:        defaultProcessConfig,
			StoragenodeUpdater: defaultProcessConfig,
			Uplink:             defaultProcessConfig,
			Gateway:            defaultProcessConfig,
			Identity:           defaultProcessConfig,
		},
	}
	if planet.config.Reconfigure.VersionControl != nil {
		planet.config.Reconfigure.VersionControl(config)
	}
	peer, err = versioncontrol.New(log, config)
	if err != nil {
		return nil, err
	}

	log.Debug(" addr= " + peer.Addr())

	return peer, nil
}

// NewVersionInfo returns the Version Info for this planet with tuned metrics.
func (planet *Planet) NewVersionInfo() version.Info {
	ver, err := version.NewSemVer("v0.0.1")
	if err != nil {
		panic(err)
	}

	info := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "testplanet",
		Version:    ver,
		Release:    false,
	}
	return info
}

// NewVersionConfig returns the Version Config for this planet with tuned metrics.
func (planet *Planet) NewVersionConfig() checker.Config {
	config := checker.Config{
		CheckInterval: defaultInterval,
	}

	config.ServerAddress = fmt.Sprintf("http://%s/", planet.VersionControl.Addr())
	config.RequestTimeout = time.Second * 15
	return config
}
