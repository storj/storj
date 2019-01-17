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
	"sort"
	"strconv"
	"strings"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/internal/processgroup"
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
		fmt.Println("sim | exec: rm -rf", flags.Directory)
	}
	return os.RemoveAll(flags.Directory)
}

// newNetwork creates a default network
func newNetwork(flags *Flags) (*Processes, error) {
	// with common adds all common arguments to the process
	withCommon := func(all Arguments) Arguments {
		for command, args := range all {
			all[command] = append([]string{
				"--log.level", "debug",
				"--config-dir", ".",
				command,
			}, args...)
		}
		return all
	}

	processes := NewProcesses()
	var (
		configDir       = flags.Directory
		host            = flags.Host
		gatewayPort     = 9000
		bootstrapPort   = 9999
		satellitePort   = 10000
		storageNodePort = 11000
	)

	bootstrap := processes.New(Info{
		Name:       "bootstrap/0",
		Executable: "bootstrap",
		Directory:  filepath.Join(configDir, "bootstrap", "0"),
		Address:    net.JoinHostPort(host, strconv.Itoa(bootstrapPort)),
	})

	bootstrap.Arguments = withCommon(Arguments{
		"run": {
			"--kademlia.bootstrap-addr", bootstrap.Address,
			"--kademlia.operator.email", "bootstrap@example.com",
			"--kademlia.operator.wallet", "0x0123456789012345678901234567890123456789",
			"--server.address", bootstrap.Address,
		},
	})

	// Create satellites making the first satellite bootstrap
	var satellites []*Process
	for i := 0; i < flags.SatelliteCount; i++ {
		process := processes.New(Info{
			Name:       fmt.Sprintf("satellite/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(configDir, "satellite", fmt.Sprint(i)),
			Address:    net.JoinHostPort(host, strconv.Itoa(satellitePort+i)),
		})
		satellites = append(satellites, process)

		// satellite must wait for bootstrap to start
		process.WaitForStart(bootstrap)

		process.Arguments = withCommon(Arguments{
			"run": {
				"--kademlia.bootstrap-addr", bootstrap.Address,
				"--server.address", process.Address,

				"--audit.satellite-addr", process.Address,
				"--repairer.overlay-addr", process.Address,
				"--repairer.pointer-db-addr", process.Address,
			},
		})
	}

	// Create gateways for each satellite
	for i, satellite := range satellites {
		accessKey, secretKey := randomKey(), randomKey()
		process := processes.New(Info{
			Name:       fmt.Sprintf("gateway/%d", i),
			Executable: "gateway",
			Directory:  filepath.Join(configDir, "gateway", fmt.Sprint(i)),
			Address:    net.JoinHostPort(host, strconv.Itoa(gatewayPort+i)),
			Extra: []string{
				"ACCESS_KEY=" + accessKey,
				"SECRET_KEY=" + secretKey,
			},
		})

		// gateway must wait for the corresponding satellite to start up
		process.WaitForStart(satellite)

		process.Arguments = withCommon(Arguments{
			"run": {
				"--server.address", process.Address,
				"--minio.access-key", accessKey,
				"--minio.secret-key", secretKey,

				"--client.overlay-addr", satellite.Address,
				"--client.pointer-db-addr", satellite.Address,

				"--rs.min-threshold", strconv.Itoa(1 * flags.StorageNodeCount / 5),
				"--rs.repair-threshold", strconv.Itoa(2 * flags.StorageNodeCount / 5),
				"--rs.success-threshold", strconv.Itoa(3 * flags.StorageNodeCount / 5),
				"--rs.max-threshold", strconv.Itoa(4 * flags.StorageNodeCount / 5),
			},
		})
	}

	// Create storage nodes
	for i := 0; i < flags.StorageNodeCount; i++ {
		process := processes.New(Info{
			Name:       fmt.Sprintf("storagenode/%d", i),
			Executable: "storagenode",
			Directory:  filepath.Join(configDir, "storage", fmt.Sprint(i)),
			Address:    net.JoinHostPort(host, strconv.Itoa(storageNodePort+i)),
		})

		// storage node must wait for bootstrap to start
		process.WaitForStart(bootstrap)

		process.Arguments = withCommon(Arguments{
			"run": {
				"--kademlia.bootstrap-addr", bootstrap.Address,
				"--kademlia.operator.email", fmt.Sprintf("storage%d@example.com", i),
				"--kademlia.operator.wallet", "0x0123456789012345678901234567890123456789",
				"--server.address", process.Address,
			},
		})
	}

	{ // verify that we have all binaries
		missing := map[string]bool{}
		for _, process := range processes.List {
			_, err := exec.LookPath(process.Executable)
			if err != nil {
				missing[process.Executable] = true
			}
		}
		if len(missing) > 0 {
			var list []string
			for executable := range missing {
				list = append(list, executable)
			}
			sort.Strings(list)
			return nil, fmt.Errorf("some executables cannot be found: %v", list)
		}
	}

	// Create directories for all processes
	for _, process := range processes.List {
		if err := os.MkdirAll(process.Directory, folderPermissions); err != nil {
			return nil, err
		}
	}

	return processes, nil
}

func randomKey() string {
	var data [10]byte
	_, _ = rand.Read(data[:])
	return hex.EncodeToString(data[:])
}
