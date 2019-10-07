// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"storj.io/storj/internal/version"
	"storj.io/storj/versioncontrol"
)

// newVersionControlServer initializes the Versioning Server
func (planet *Planet) newVersionControlServer() (peer *versioncontrol.Peer, err error) {

	prefix := "versioncontrol"
	log := planet.log.Named(prefix)
	dbDir := filepath.Join(planet.directory, prefix)

	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, err
	}

	config := &versioncontrol.Config{
		Address: "127.0.0.1:0",
		Versions: versioncontrol.ServiceVersions{
			Satellite:   "v0.0.1",
			Storagenode: "v0.0.1",
			Uplink:      "v0.0.1",
			Gateway:     "v0.0.1",
			Identity:    "v0.0.1",
		},
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
	info := version.Info{
		Timestamp:  time.Now(),
		CommitHash: "testplanet",
		Version: version.SemVer{
			Major: 0,
			Minor: 0,
			Patch: 1},
		Release: false,
	}
	return info
}

// NewVersionConfig returns the Version Config for this planet with tuned metrics.
func (planet *Planet) NewVersionConfig() version.Config {
	return version.Config{
		ServerAddress:  fmt.Sprintf("http://%s/", planet.VersionControl.Addr()),
		RequestTimeout: time.Second * 15,
		CheckInterval:  time.Minute * 5,
	}
}
