// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/piecestore/psservice"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run all providers",
		RunE:  cmdRun,
	}
	runCfg Config
)

func init() {
	rootCmd.AddCommand(runCmd)
	cfgstruct.Bind(runCmd.Flags(), &runCfg,
		cfgstruct.ConfDir(defaultConfDir),
	)
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)
	defer mon.Task()(&ctx)(&err)

	startingPort := runCfg.StartingPort

	errch := make(chan error, runCfg.FarmerCount+2)

	// define heavy client config programmatically
	type HeavyClient struct {
		Identity  provider.IdentityConfig
		Kademlia  kademlia.Config
		PointerDB pointerdb.Config
		Overlay   overlay.Config
	}

	hc := HeavyClient{
		Identity: provider.IdentityConfig{
			CertPath: filepath.Join(runCfg.BasePath, "hc", "ident.leaf.cert"),
			KeyPath:  filepath.Join(runCfg.BasePath, "hc", "ident.leaf.key"),
			Address:  joinHostPort(runCfg.ListenHost, startingPort+1),
		},
		Kademlia: kademlia.Config{
			TODOListenAddr: joinHostPort(runCfg.ListenHost, startingPort+2),
			BootstrapAddr:  joinHostPort(runCfg.ListenHost, startingPort+4),
		},
		PointerDB: pointerdb.Config{
			DatabaseURL: "bolt://" + filepath.Join(
				runCfg.BasePath, "hc", "pointerdb.db"),
		},
		Overlay: overlay.Config{
			DatabaseURL: "bolt://" + filepath.Join(
				runCfg.BasePath, "hc", "overlay.db"),
		},
	}

	// start heavy client
	go func() {
		_, _ = fmt.Printf("starting heavy client on %s\n", hc.Identity.Address)
		errch <- hc.Identity.Run(ctx, hc.Kademlia, hc.PointerDB, hc.Overlay)
	}()

	// define and start a bunch of farmers programmatically
	type Farmer struct {
		Identity provider.IdentityConfig
		Kademlia kademlia.Config
		Storage  psservice.Config
	}

	for i := 0; i < runCfg.FarmerCount; i++ {
		basepath := filepath.Join(runCfg.BasePath, fmt.Sprintf("f%d", i))
		farmer := Farmer{
			Identity: provider.IdentityConfig{
				CertPath: filepath.Join(basepath, "ident.leaf.cert"),
				KeyPath:  filepath.Join(basepath, "ident.leaf.key"),
				Address:  joinHostPort(runCfg.ListenHost, startingPort+i*2+3),
			},
			Kademlia: kademlia.Config{
				TODOListenAddr: joinHostPort(runCfg.ListenHost, startingPort+i*2+4),
				BootstrapAddr:  joinHostPort(runCfg.ListenHost, startingPort+1),
			},
			Storage: psservice.Config{
				Path: filepath.Join(basepath, "data"),
			},
		}
		go func(i int) {
			_, _ = fmt.Printf("starting farmer %d grpc on %s, kad on %s\n",
				i, farmer.Identity.Address, farmer.Kademlia.TODOListenAddr)
			errch <- farmer.Identity.Run(ctx, farmer.Kademlia, farmer.Storage)
		}(i)
	}

	// start s3 gateway
	gw := miniogw.Config{
		IdentityConfig: provider.IdentityConfig{
			CertPath: filepath.Join(runCfg.BasePath, "gw", "ident.leaf.cert"),
			KeyPath:  filepath.Join(runCfg.BasePath, "gw", "ident.leaf.key"),
			Address:  joinHostPort(runCfg.ListenHost, startingPort),
		},
		MinioConfig: runCfg.MinioConfig,
		ClientConfig: miniogw.ClientConfig{
			OverlayAddr: joinHostPort(
				runCfg.ListenHost, startingPort+1),
			PointerDBAddr: joinHostPort(
				runCfg.ListenHost, startingPort+1),
		},
		RSConfig: runCfg.RSConfig,
	}
	gw.MinioConfig.MinioDir = filepath.Join(runCfg.BasePath, "gw", "minio")

	// start s3 gateway
	go func() {
		_, _ = fmt.Printf("starting minio gateway on %s\n",
			gw.IdentityConfig.Address)
		errch <- gw.Run(ctx)
	}()

	return <-errch
}

func joinHostPort(host string, port int) string {
	return net.JoinHostPort(host, fmt.Sprint(port))
}
