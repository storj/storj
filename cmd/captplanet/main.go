// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/accounting/rollup"
	"storj.io/storj/pkg/accounting/tally"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/datarepair/checker"
	"storj.io/storj/pkg/datarepair/repairer"
	"storj.io/storj/pkg/discovery"
	"storj.io/storj/pkg/inspector"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/piecestore/psserver"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/satellite/satelliteweb"
	"storj.io/storj/pkg/server"
)

var (
	mon = monkit.Package()

	rootCmd = &cobra.Command{
		Use:   "captplanet",
		Short: "Captain Planet! With our powers combined!",
	}

	defaultConfDir = fpath.ApplicationDir("storj", "capt")
	confDir        *string
)

// Config defines broad Captain Planet configuration
type Config struct {
	SatelliteCA         provider.CASetupConfig
	SatelliteIdentity   provider.IdentitySetupConfig
	UplinkCA            provider.CASetupConfig
	UplinkIdentity      provider.IdentitySetupConfig
	StorageNodeCA       provider.CASetupConfig
	StorageNodeIdentity provider.IdentitySetupConfig
	ListenHost          string `help:"the host for providers to listen on" default:"127.0.0.1"`
	StartingPort        int    `help:"all providers will listen on ports consecutively starting with this one" default:"7777"`
	APIKey              string `default:"abc123" help:"the api key to use for the satellite"`
	EncKey              string `default:"insecure-default-encryption-key" help:"your root encryption key"`
	Overwrite           bool   `help:"whether to overwrite pre-existing configuration files" default:"false"`
	GenerateMinioCerts  bool   `default:"false" help:"generate sample TLS certs for Minio GW"`

	Satellite    Satellite
	StorageNodes [storagenodeCount]StorageNode
	Uplink       miniogw.Config
}

// Satellite is for configuring client
type Satellite struct {
	Server      server.Config
	Kademlia    kademlia.SatelliteConfig
	PointerDB   pointerdb.Config
	Overlay     overlay.Config
	Inspector   inspector.Config
	Checker     checker.Config
	Repairer    repairer.Config
	Audit       audit.Config
	BwAgreement bwagreement.Config
	Web         satelliteweb.Config
	Database    string `help:"satellite database connection string" default:"sqlite3://$CONFDIR/master.db"`
	Discovery   discovery.Config
	Tally       tally.Config
	Rollup      rollup.Config
}

// StorageNode is for configuring storage nodes
type StorageNode struct {
	Server   server.Config
	Kademlia kademlia.StorageNodeConfig
	Storage  psserver.Config
}

func main() {
	go dumpHandler()
	process.Exec(rootCmd)
}

func init() {
	dirParam := cfgstruct.FindConfigDirParam()
	if dirParam != "" {
		defaultConfDir = dirParam
	}

	confDir = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for captplanet configuration")
}

// dumpHandler listens for Ctrl+\ on Unix
func dumpHandler() {
	if runtime.GOOS == "windows" {
		// unsupported on Windows
		return
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGQUIT)
	for range sigs {
		dumpGoroutines()
	}
}

func dumpGoroutines() {
	buf := make([]byte, memory.MB)
	n := runtime.Stack(buf, true)

	p := time.Now().Format("dump-2006-01-02T15-04-05.999999999.log")
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	fmt.Fprintf(os.Stderr, "Writing stack traces to \"%v\"\n", p)

	err := ioutil.WriteFile(p, buf[:n], 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
	}
}
