// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"storj.io/storj/bootstrap"
	"storj.io/storj/bootstrap/bootstrapdb"
	"storj.io/storj/bootstrap/bootstrapweb/bootstrapserver"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/versioncontrol"
)

// newBootstrap initializes the bootstrap node
func (planet *Planet) newBootstrap() (peer *bootstrap.Peer, err error) {
	defer func() {
		planet.peers = append(planet.peers, closablePeer{peer: peer})
	}()

	prefix := "bootstrap"
	log := planet.log.Named(prefix)
	dbDir := filepath.Join(planet.directory, prefix)

	if err := os.MkdirAll(dbDir, 0700); err != nil {
		return nil, err
	}

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, err
	}

	var db bootstrap.DB
	if planet.config.Reconfigure.NewBootstrapDB != nil {
		db, err = planet.config.Reconfigure.NewBootstrapDB(0)
	} else {
		db, err = bootstrapdb.NewInMemory()
	}

	if err != nil {
		return nil, err
	}

	err = db.CreateTables()
	if err != nil {
		return nil, err
	}

	planet.databases = append(planet.databases, db)

	config := bootstrap.Config{
		Server: server.Config{
			Address:        "127.0.0.1:0",
			PrivateAddress: "127.0.0.1:0",

			Config: tlsopts.Config{
				RevocationDBURL:     "bolt://" + filepath.Join(dbDir, "revocation.db"),
				UsePeerCAWhitelist:  true,
				PeerCAWhitelistPath: planet.whitelistPath,
				PeerIDVersions:      "latest",
				Extensions: extensions.Config{
					Revocation:          false,
					WhitelistSignedLeaf: false,
				},
			},
		},
		Kademlia: kademlia.Config{
			BootstrapBackoffBase: 500 * time.Millisecond,
			BootstrapBackoffMax:  2 * time.Second,
			Alpha:                5,
			DBPath:               dbDir, // TODO: replace with master db
			Operator: kademlia.OperatorConfig{
				Email:  prefix + "@mail.test",
				Wallet: "0x" + strings.Repeat("00", 20),
			},
		},
		Web: bootstrapserver.Config{
			Address:   "127.0.0.1:0",
			StaticDir: "./web/bootstrap", // TODO: for development only
		},
		Version: planet.NewVersionConfig(),
	}
	if planet.config.Reconfigure.Bootstrap != nil {
		planet.config.Reconfigure.Bootstrap(0, &config)
	}

	var verInfo version.Info
	verInfo = planet.NewVersionInfo()

	peer, err = bootstrap.New(log, identity, db, config, verInfo)
	if err != nil {
		return nil, err
	}

	log.Debug("id=" + peer.ID().String() + " addr=" + peer.Addr())

	return peer, nil
}

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
			Bootstrap:   "v0.0.1",
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
