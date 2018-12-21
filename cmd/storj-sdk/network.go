// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/processgroup"
	"storj.io/storj/pkg/utils"
)

func networkExec(flags *Flags, args []string, command string) error {
	processes, err := newNetwork(flags.Directory, flags.SatelliteCount, flags.StorageNodeCount)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	err = processes.Exec(ctx, command)
	closeErr := processes.Close()

	return errs.Combine(err, closeErr)
}

func networkTest(flags *Flags, command string, args []string) error {
	processes, err := newNetwork(flags.Directory, flags.SatelliteCount, flags.StorageNodeCount)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())

	var group errgroup.Group
	processes.Start(ctx, &group, command)

	time.Sleep(2 * time.Second)

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = processes.Env()
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	processgroup.Setup(cmd)

	errRun := cmd.Run()

	cancel()
	return errs.Combine(errRun, processes.Close(), group.Wait())
}

func networkDestroy(flags *Flags, args []string) error {
	if fpath.IsRoot(flags.Directory) {
		return errors.New("safety check: disallowed to remove root directory " + flags.Directory)
	}

	fmt.Println("exec: rm -rf", flags.Directory)
	return os.RemoveAll(flags.Directory)
}

// newNetwork creates a default network
func newNetwork(dir string, satelliteCount, storageNodeCount int) (*Processes, error) {
	processes := &Processes{}

	const (
		host            = "127.0.0.1"
		gatewayPort     = 9000
		satellitePort   = 10000
		storageNodePort = 11000
	)

	defaultSatellite := net.JoinHostPort(host, strconv.Itoa(satellitePort+0))

	arguments := func(name, command string, port int, rest ...string) []string {
		return append([]string{
			"--log.level", "debug",
			"--log.prefix", name,
			"--config-dir", ".",
			command,
			"--identity.server.address", net.JoinHostPort(host, strconv.Itoa(port)),
		}, rest...)
	}

	for i := 0; i < satelliteCount; i++ {
		name := fmt.Sprintf("satellite/%d", i)

		dir := filepath.Join(dir, "satellite", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process, err := NewProcess(name, "satellite", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		processes.List = append(processes.List, process)

		process.Arguments["setup"] = arguments(name, "setup", satellitePort+i)
		process.Arguments["run"] = arguments(name,
			"run", satellitePort+i,
			"--kademlia.bootstrap-addr", defaultSatellite,
		)
	}

	gatewayArguments := func(name, command string, index int, rest ...string) []string {
		return append([]string{
			"--log.level", "debug",
			"--log.prefix", name,
			"--config-dir", ".",
			command,
			// "--satellite-addr", net.JoinHostPort(host, strconv.Itoa(satellitePort+index)),
			"--identity.server.address", net.JoinHostPort(host, strconv.Itoa(gatewayPort+index)),
		}, rest...)
	}

	for i := 0; i < satelliteCount; i++ {
		name := fmt.Sprintf("gateway/%d", i)

		dir := filepath.Join(dir, "gateway", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process, err := NewProcess(name, "gateway", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		processes.List = append(processes.List, process)

		process.Arguments["setup"] = gatewayArguments(name, "setup", i)
		process.Arguments["run"] = gatewayArguments(name, "run", i)
	}

	for i := 0; i < storageNodeCount; i++ {
		name := fmt.Sprintf("storage/%d", i)

		dir := filepath.Join(dir, "storagenode", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process, err := NewProcess(name, "storagenode", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		processes.List = append(processes.List, process)

		process.Arguments["setup"] = arguments(name, "setup", storageNodePort+i,
			"--piecestore.agreementsender.overlay-addr", defaultSatellite,
		)
		process.Arguments["run"] = arguments(name, "run", storageNodePort+i,
			"--piecestore.agreementsender.overlay-addr", defaultSatellite,
			"--kademlia.bootstrap-addr", defaultSatellite,
		)
	}

	return processes, nil
}
