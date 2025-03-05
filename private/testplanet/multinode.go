// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strconv"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/debug"
	"storj.io/common/storj"
	"storj.io/storj/multinode"
	"storj.io/storj/multinode/console/server"
	"storj.io/storj/multinode/multinodedb"
)

// Multinode contains all the processes needed to run a full multinode setup.
type Multinode struct {
	Name   string
	Config multinode.Config
	*multinode.Peer
}

// ID returns multinode id.
func (system *Multinode) ID() storj.NodeID { return system.Identity.ID }

// Addr returns the public address.
func (system *Multinode) Addr() string { return system.Console.Listener.Addr().String() }

// Label returns name for debugger.
func (system *Multinode) Label() string { return system.Name }

// URL returns the NodeURL as a string.
func (system *Multinode) URL() string { return system.NodeURL().String() }

// NodeURL returns the storj.NodeURL.
func (system *Multinode) NodeURL() storj.NodeURL {
	return storj.NodeURL{ID: system.ID(), Address: system.Addr()}
}

// ConsoleURL returns the console URL.
func (system *Multinode) ConsoleURL() string {
	return "http://" + system.Addr()
}

// newMultinodes initializes multinode dashboards.
func (planet *Planet) newMultinodes(ctx context.Context, prefix string, count int) (_ []*Multinode, err error) {
	defer mon.Task()(&ctx)(&err)

	var xs []*Multinode
	for i := 0; i < count; i++ {
		index := i
		name := prefix + strconv.Itoa(index)
		log := planet.log.Named(name)

		var system *Multinode
		var err error
		pprof.Do(ctx, pprof.Labels("peer", name), func(ctx context.Context) {
			system, err = planet.newMultinode(ctx, name, index, log)
		})
		if err != nil {
			return nil, errs.Wrap(err)
		}

		log.Debug("id=" + system.ID().String() + " addr=" + system.Addr())
		xs = append(xs, system)
		planet.peers = append(planet.peers, newClosablePeer(system))
	}
	return xs, nil
}

func (planet *Planet) newMultinode(ctx context.Context, prefix string, index int, log *zap.Logger) (_ *Multinode, err error) {
	defer mon.Task()(&ctx)(&err)

	storageDir := filepath.Join(planet.directory, prefix)
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, errs.Wrap(err)
	}

	identity, err := planet.NewIdentity()
	if err != nil {
		return nil, errs.Wrap(err)
	}

	config := multinode.Config{
		Debug: debug.Config{
			Addr: "",
		},
		Console: server.Config{
			Address:   net.JoinHostPort(planet.config.Host, "0"),
			StaticDir: filepath.Join(developmentRoot, "web/multinode/"),
		},
	}
	if planet.config.Reconfigure.Multinode != nil {
		planet.config.Reconfigure.Multinode(index, &config)
	}

	database := fmt.Sprintf("sqlite3://file:%s/master.db", storageDir)

	var db multinode.DB
	db, err = multinodedb.Open(ctx, log.Named("db"), database)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if planet.config.Reconfigure.MultinodeDB != nil {
		db, err = planet.config.Reconfigure.MultinodeDB(index, db, planet.log)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	peer, err := multinode.New(log, identity, config, db)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	planet.databases = append(planet.databases, db)

	log.Debug(peer.Console.Listener.Addr().String())

	return &Multinode{
		Name:   prefix,
		Config: config,
		Peer:   peer,
	}, nil
}
