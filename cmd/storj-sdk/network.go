// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/processgroup"
	"storj.io/storj/pkg/utils"
)

const folderPermissions = 0744

func networkExec(flags *Flags, args []string, command string) error {
	processes, err := newNetwork(flags)
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
	processes, err := newNetwork(flags)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())

	var group errgroup.Group
	processes.Start(ctx, &group, "run")

	for _, process := range processes.List {
		process.Status.Started.Wait()
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(os.Environ(), processes.Env()...)
	stdout := processes.Output.Prefixed("test:out")
	stderr := processes.Output.Prefixed("test:err")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	processgroup.Setup(cmd)

	if printCommands {
		fmt.Fprintf(processes.Output, "exec: %v\n", strings.Join(cmd.Args, " "))
	}
	errRun := cmd.Run()

	cancel()
	return errs.Combine(errRun, processes.Close(), group.Wait())
}

func networkDestroy(flags *Flags, args []string) error {
	if fpath.IsRoot(flags.Directory) {
		return errors.New("safety check: disallowed to remove root directory " + flags.Directory)
	}
	if printCommands {
		fmt.Println("sdk | exec: rm -rf", flags.Directory)
	}
	return os.RemoveAll(flags.Directory)
}

// newNetwork creates a default network
func newNetwork(flags *Flags) (*Processes, error) {
	processes := NewProcesses()

	var (
		configDir       = flags.Directory
		host            = flags.Host
		gatewayPort     = 9000
		satellitePort   = 10000
		storageNodePort = 11000
	)

	var bootstrapSatellite *Process

	arguments := func(name, command, addr string, rest ...string) []string {
		return append([]string{
			"--log.level", "debug",
			"--config-dir", ".",
			command,
		}, rest...)
	}

	for i := 0; i < flags.SatelliteCount; i++ {
		name := fmt.Sprintf("satellite/%d", i)

		dir := filepath.Join(configDir, "satellite", fmt.Sprint(i))
		if err := os.MkdirAll(dir, folderPermissions); err != nil {
			return nil, err
		}

		process, err := processes.New(name, "satellite", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}

		process.Info.Address = net.JoinHostPort(host, strconv.Itoa(satellitePort+i))

		bootstrapAddr := process.Info.Address
		if bootstrapSatellite != nil {
			bootstrapAddr = bootstrapSatellite.Info.Address
			process.WaitForStart(bootstrapSatellite)
		} else {
			bootstrapSatellite = process
		}

		process.Arguments["setup"] = arguments(name, "setup", process.Info.Address)
		process.Arguments["run"] = arguments(name, "run", process.Info.Address,
			"--kademlia.bootstrap-addr", bootstrapAddr,
			"--server.address", process.Info.Address,
		)
	}

	gatewayArguments := func(name, command string, addr string, rest ...string) []string {
		return append([]string{
			"--log.level", "debug",
			"--config-dir", ".",
			command,
		}, rest...)
	}

	for i := 0; i < flags.SatelliteCount; i++ {
		name := fmt.Sprintf("gateway/%d", i)

		dir := filepath.Join(configDir, "gateway", fmt.Sprint(i))
		if err := os.MkdirAll(dir, folderPermissions); err != nil {
			return nil, err
		}

		satellite := processes.List[i]

		process, err := processes.New(name, "gateway", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		process.Info.Address = net.JoinHostPort(host, strconv.Itoa(gatewayPort+i))

		process.WaitForStart(satellite)

		process.Arguments["setup"] = gatewayArguments(name, "setup", process.Info.Address,
			"--satellite-addr", satellite.Info.Address,
		)

		accessKey, secretKey := randomKey(), randomKey()
		process.Arguments["run"] = gatewayArguments(name, "run", process.Info.Address,
			"--server.address", process.Info.Address,
			"--minio.access-key", accessKey,
			"--minio.secret-key", secretKey,

			"--client.overlay-addr", satellite.Info.Address,
			"--client.pointer-db-addr", satellite.Info.Address,
		)

		process.Info.Extra = []string{
			"ACCESS_KEY=" + accessKey,
			"SECRET_KEY=" + secretKey,
		}
	}

	for i := 0; i < flags.StorageNodeCount; i++ {
		name := fmt.Sprintf("storage/%d", i)

		dir := filepath.Join(configDir, "storage", fmt.Sprint(i))
		if err := os.MkdirAll(dir, folderPermissions); err != nil {
			return nil, err
		}

		process, err := processes.New(name, "storagenode", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		process.Info.Address = net.JoinHostPort(host, strconv.Itoa(storageNodePort+i))

		process.WaitForStart(bootstrapSatellite)

		process.Arguments["setup"] = arguments(name, "setup", process.Info.Address,
			"--piecestore.agreementsender.overlay-addr", bootstrapSatellite.Info.Address,
		)
		process.Arguments["run"] = arguments(name, "run", process.Info.Address,
			"--piecestore.agreementsender.overlay-addr", bootstrapSatellite.Info.Address,
			"--kademlia.bootstrap-addr", bootstrapSatellite.Info.Address,
			"--kademlia.operator.email", fmt.Sprintf("storage%d@example.com", i),
			"--kademlia.operator.wallet", "0x0123456789012345678901234567890123456789",
			"--server.address", process.Info.Address,
		)
	}

	return processes, nil
}

func randomKey() string {
	var data [10]byte
	_, _ = rand.Read(data[:])
	return hex.EncodeToString(data[:])
}
