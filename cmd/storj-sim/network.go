// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alessio/shellescape"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/base58"
	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/shared/dbutil"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/processgroup"
	"storj.io/uplink"
)

const (
	maxInstanceCount    = 100
	maxStoragenodeCount = 200

	folderPermissions = 0744

	gatewayGracePeriod = 10 * time.Second
)

var defaultAccess = "12edqtGZnqQo6QHwTB92EDqg9B1WrWn34r7ALu94wkqXL4eXjBNnVr6F5W7GhJjVqJCqxpFERmDR1dhZWyMt3Qq5zwrE9yygXeT6kBoS9AfiPuwB6kNjjxepg5UtPPtp4VLp9mP5eeyobKQRD5TsEsxTGhxamsrHvGGBPrZi8DeLtNYFMRTV6RyJVxpYX6MrPCw9HVoDQbFs7VcPeeRxRMQttSXL3y33BJhkqJ6ByFviEquaX5R2wjQT2Kx"

const (
	// The following values of peer class and endpoints are used
	// to create a port with a consistent format for storj-sim services.

	// Peer classes.
	satellitePeer       = 0
	satellitePeerWorker = 4
	gatewayPeer         = 1
	versioncontrolPeer  = 2
	storagenodePeer     = 3
	multinodePeer       = 5
	rangedloopPeer      = 6

	// Endpoints.
	publicRPC  = 0
	privateRPC = 1
	publicHTTP = 2
	debugHTTP  = 9

	// Satellite specific constants.
	redisPort      = 4
	adminHTTP      = 5
	debugAdminHTTP = 6
	debugCoreHTTP  = 7

	// Satellite worker specific constants.
	debugMigrationHTTP = 0
	debugRepairerHTTP  = 1
	debugGCHTTP        = 2
)

// port creates a port with a consistent format for storj-sim services.
// The port format is: "1PXXE", where P is the peer class, XX is the index of the instance, and E is the endpoint.
func port(peerclass, index, endpoint int) string {
	port := 10000 + peerclass*1000 + index*10 + endpoint
	return strconv.Itoa(port)
}

func networkExec(flags *Flags, args []string, command string) error {
	processes, err := newNetwork(flags)
	if err != nil {
		return err
	}
	defer func() { _ = processes.Output.Flush() }()

	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	if command == "setup" {
		if flags.Postgres == "" {
			return errors.New("postgres connection URL is required for running storj-sim. Example: `storj-sim network setup --postgres=<connection URL>`.\nSee docs for more details https://github.com/storj/docs/blob/main/Test-network.md#running-tests-with-postgres")
		}

		identities, err := identitySetup(processes)
		if err != nil {
			return err
		}

		err = identities.Exec(ctx, command)
		if err != nil {
			return err
		}
	}

	err = processes.Exec(ctx, command)
	closeErr := processes.Close()

	return errs.Combine(err, closeErr)
}

func escapeEnv(env string) string {
	// TODO(jeff): escape env variables appropriately on windows. perhaps the
	// env output should be of the form `set KEY=VALUE` as well.
	if runtime.GOOS == "windows" {
		return env
	}

	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return env
	}
	return parts[0] + "=" + shellescape.Quote(parts[1])
}

func networkEnv(flags *Flags, args []string) error {
	flags.OnlyEnv = true

	processes, err := newNetwork(flags)
	if err != nil {
		return err
	}
	defer func() { _ = processes.Output.Flush() }()

	// run exec before, since it will load env vars from configs
	for _, process := range processes.List {
		if exec := process.ExecBefore["run"]; exec != nil {
			if err := exec(process); err != nil {
				return err
			}
		}
	}

	if len(args) == 1 {
		envprefix := strings.ToUpper(args[0] + "=")
		// find the environment value that the environment variable is set to
		for _, env := range processes.Env() {
			if strings.HasPrefix(strings.ToUpper(env), envprefix) {
				fmt.Println(escapeEnv(env[len(envprefix):]))
				return nil
			}
		}

		return nil
	}

	for _, env := range processes.Env() {
		fmt.Println(escapeEnv(env))
	}

	return nil
}

func networkTest(flags *Flags, command string, args []string) error {
	processes, err := newNetwork(flags)
	if err != nil {
		return err
	}
	defer func() { _ = processes.Output.Flush() }()

	ctx, cancel := NewCLIContext(context.Background())

	var group *errgroup.Group
	if processes.FailFast {
		group, ctx = errgroup.WithContext(ctx)
	} else {
		group = &errgroup.Group{}
	}

	processes.Start(ctx, group, "run")

	for _, process := range processes.List {
		process.Status.Started.Wait(ctx)
	}
	if err := ctx.Err(); err != nil {
		// If the context has been cancelled, it means that one of the processes failed.
		// Wait for the processes to shut down themselves and return the first error.
		return fmt.Errorf("network canceled: %w", group.Wait())
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = append(os.Environ(), processes.Env()...)

	stdout := processes.Output.Prefixed("test:out")
	defer func() { _ = stdout.Flush() }()
	stderr := processes.Output.Prefixed("test:err")
	defer func() { _ = stderr.Flush() }()
	cmd.Stdout, cmd.Stderr = stdout, stderr

	processgroup.Setup(cmd)

	if printCommands {
		_, _ = fmt.Fprintf(processes.Output, "exec: %v\n", strings.Join(cmd.Args, " "))
	}
	errRun := cmd.Run()
	if errRun != nil {
		_, _ = fmt.Fprintf(processes.Output, "test command failed: %v\n", errRun)
	}

	cancel()
	_ = group.Wait()
	return errs.Combine(errRun, processes.Close())
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

// newNetwork creates a default network.
func newNetwork(flags *Flags) (*Processes, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return nil, errs.New("no caller information")
	}
	storjRoot := strings.TrimSuffix(filename, "/cmd/storj-sim/network.go")

	// with common adds all common arguments to the process
	withCommon := func(dir string, all Arguments) Arguments {
		common := []string{"--metrics.app-suffix", "sim", "--log.level", "debug", "--config-dir", dir}
		if flags.IsDev {
			common = append(common, "--defaults", "dev")
		} else {
			common = append(common, "--defaults", "release")
		}
		for command, args := range all {
			full := append([]string{}, common...)
			full = append(full, command)
			full = append(full, args...)
			all[command] = full
		}
		return all
	}

	processes := NewProcesses(flags.Directory, flags.FailFast)

	host := flags.Host
	versioncontrol := processes.New(Info{
		Name:       "versioncontrol/0",
		Executable: "versioncontrol",
		Directory:  filepath.Join(processes.Directory, "versioncontrol", "0"),
		Address:    net.JoinHostPort(host, port(versioncontrolPeer, 0, publicRPC)),
	})

	versioncontrol.Arguments = withCommon(versioncontrol.Directory, Arguments{
		"setup": {
			"--address", versioncontrol.Address,
			"--debug.addr", net.JoinHostPort(host, port(versioncontrolPeer, 0, debugHTTP)),
			"--binary.gateway.rollout.seed", "0000000000000000000000000000000000000000000000000000000000000001",
			"--binary.identity.rollout.seed", "0000000000000000000000000000000000000000000000000000000000000001",
			"--binary.satellite.rollout.seed", "0000000000000000000000000000000000000000000000000000000000000001",
			"--binary.storagenode-updater.rollout.seed", "0000000000000000000000000000000000000000000000000000000000000001",
			"--binary.storagenode.rollout.seed", "0000000000000000000000000000000000000000000000000000000000000001",
			"--binary.uplink.rollout.seed", "0000000000000000000000000000000000000000000000000000000000000001",
		},
		"run": {},
	})

	versioncontrol.ExecBefore["run"] = func(process *Process) error {
		return readConfigString(&versioncontrol.Address, versioncontrol.Directory, "address")
	}
	// gateway must wait for the versioncontrol to start up

	// Create satellites
	if flags.SatelliteCount > maxInstanceCount {
		return nil, fmt.Errorf("exceeded the max instance count of %d with Satellite count of %d", maxInstanceCount, flags.SatelliteCount)
	}

	// set up redis servers
	var redisServers []*Process

	if flags.Redis == "" {
		for i := 0; i < flags.SatelliteCount; i++ {
			rp := port(satellitePeer, i, redisPort)
			process := processes.New(Info{
				Name:       fmt.Sprintf("redis/%d", i),
				Executable: "redis-server",
				Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i), "redis"),
				Address:    net.JoinHostPort(host, rp),
			})
			redisServers = append(redisServers, process)

			process.ExecBefore["setup"] = func(process *Process) error {
				confpath := filepath.Join(process.Directory, "redis.conf")
				arguments := []string{
					"daemonize no",
					"bind " + host,
					"port " + rp,
					"timeout 0",
					"databases 2",
					"dbfilename sim.rdb",
					"dir ./",
				}
				conf := strings.Join(arguments, "\n") + "\n"
				err := os.WriteFile(confpath, []byte(conf), 0755)
				return err
			}
			process.Arguments = Arguments{
				"run": []string{filepath.Join(process.Directory, "redis.conf")},
			}
		}
	}

	var satellites []*Process
	for i := 0; i < flags.SatelliteCount; i++ {
		apiProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
			Address:    net.JoinHostPort(host, port(satellitePeer, i, publicRPC)),
		})
		satellites = append(satellites, apiProcess)

		redisAddress := flags.Redis
		redisPortBase := flags.RedisStartDB + i*2
		if redisAddress == "" {
			redisAddress = redisServers[i].Address
			redisPortBase = 0
			apiProcess.WaitForStart(redisServers[i])
		}

		apiProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"setup": {
				"--identity-dir", apiProcess.Directory,

				"--console.address", net.JoinHostPort(host, port(satellitePeer, i, publicHTTP)),
				"--console.static-dir", filepath.Join(storjRoot, "web/satellite/"),
				"--console.auth-token-secret", "my-suppa-secret-key",
				"--console.open-registration-enabled",
				"--console.rate-limit.burst", "100",

				"--server.address", apiProcess.Address,
				"--server.private-address", net.JoinHostPort(host, port(satellitePeer, i, privateRPC)),

				"--live-accounting.storage-backend", "redis://" + redisAddress + "?db=" + strconv.Itoa(redisPortBase),
				"--server.revocation-dburl", "redis://" + redisAddress + "?db=" + strconv.Itoa(redisPortBase+1),

				"--server.extensions.revocation=false",
				"--server.use-peer-ca-whitelist=false",

				"--mail.smtp-server-address", "smtp.gmail.com:587",
				"--mail.from", "Storj <yaroslav-satellite-test@storj.io>",
				"--mail.template-path", filepath.Join(storjRoot, "web/satellite/static/emails"),
				"--version.server-address", fmt.Sprintf("http://%s/", versioncontrol.Address),
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugHTTP)),

				"--admin.address", net.JoinHostPort(host, port(satellitePeer, i, adminHTTP)),
				"--admin.static-dir", filepath.Join(storjRoot, "satellite/admin/ui/build"),
			},
			"run": {"api"},
		})

		if flags.Postgres != "" {
			masterDBURL, err := namespacedDatabaseURL(flags.Postgres, fmt.Sprintf("satellite/%d", i))
			if err != nil {
				return nil, err
			}
			metainfoDBURL, err := namespacedDatabaseURL(flags.Postgres, fmt.Sprintf("satellite/%d/meta", i))
			if err != nil {
				return nil, err
			}

			apiProcess.Arguments["setup"] = append(apiProcess.Arguments["setup"],
				"--database", masterDBURL,
				"--metainfo.database-url", metainfoDBURL,
				"--orders.encryption-keys", "0100000000000000=0100000000000000000000000000000000000000000000000000000000000000",
			)
		}
		apiProcess.ExecBefore["run"] = func(process *Process) error {
			if err := readConfigString(&process.Address, process.Directory, "server.address"); err != nil {
				return err
			}

			satNodeID, err := identity.NodeIDFromCertPath(filepath.Join(apiProcess.Directory, "identity.cert"))
			if err != nil {
				return err
			}
			process.Info.ID = satNodeID.String()
			return nil
		}

		migrationProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-migration/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
		})
		migrationProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"migration",
				"--debug.addr", net.JoinHostPort(host, port(satellitePeerWorker, i, debugMigrationHTTP)),
			},
		})
		apiProcess.WaitForExited(migrationProcess)

		coreProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-core/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
			Address:    "",
		})
		coreProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugCoreHTTP)),
				"--orders.encryption-keys", "0100000000000000=0100000000000000000000000000000000000000000000000000000000000000",
			},
		})
		coreProcess.WaitForExited(migrationProcess)

		rangedLoopProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-rangedloop/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
			Address:    "",
		})
		rangedLoopProcess.Arguments = withCommon(rangedLoopProcess.Directory, Arguments{
			"run": {
				"ranged-loop",
				"--debug.addr", net.JoinHostPort(host, port(rangedloopPeer, i, debugCoreHTTP)),
			},
		})
		rangedLoopProcess.WaitForExited(migrationProcess)

		adminProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-admin/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
			Address:    net.JoinHostPort(host, port(satellitePeer, i, adminHTTP)),
		})
		adminProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"admin",
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugAdminHTTP)),
			},
		})
		adminProcess.WaitForExited(migrationProcess)

		repairProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-repairer/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
		})
		repairProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"repair",
				"--debug.addr", net.JoinHostPort(host, port(satellitePeerWorker, i, debugRepairerHTTP)),
				"--orders.encryption-keys", "0100000000000000=0100000000000000000000000000000000000000000000000000000000000000",
			},
		})
		repairProcess.WaitForExited(migrationProcess)

		garbageCollectionProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-garbage-collection/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", strconv.Itoa(i)),
		})
		garbageCollectionProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"garbage-collection",
				"--debug.addr", net.JoinHostPort(host, port(satellitePeerWorker, i, debugGCHTTP)),
			},
		})
		garbageCollectionProcess.WaitForExited(migrationProcess)
	}

	// Create gateways for each satellite
	for i, satellite := range satellites {
		if flags.NoGateways {
			break
		}
		satellite := satellite
		process := processes.New(Info{
			Name:       fmt.Sprintf("gateway/%d", i),
			Executable: "gateway",
			Directory:  filepath.Join(processes.Directory, "gateway", strconv.Itoa(i)),
			Address:    net.JoinHostPort(host, port(gatewayPeer, i, publicRPC)),
		})

		// gateway must wait for the corresponding satellite to start up
		process.WaitForStart(satellite)

		accessData := defaultAccess

		process.Arguments = withCommon(process.Directory, Arguments{
			"setup": {
				"--non-interactive",

				"--access", accessData,
				"--server.address", process.Address,

				"--debug.addr", net.JoinHostPort(host, port(gatewayPeer, i, debugHTTP)),
			},

			"run": {},
		})

		process.ExecBefore["run"] = func(process *Process) (err error) {
			err = readConfigString(&process.Address, process.Directory, "server.address")
			if err != nil {
				return err
			}

			vip := viper.New()
			vip.AddConfigPath(process.Directory)
			if err := vip.ReadInConfig(); err != nil {
				return err
			}

			// TODO: maybe all the config flags should be exposed for all processes?

			// check if gateway config has an api key, if it's not
			// create example project with key and add it to the config
			// so that gateway can have access to the satellite
			if runAccessData := vip.GetString("access"); !flags.OnlyEnv && runAccessData == accessData {
				var consoleAddress string
				err := readConfigString(&consoleAddress, satellite.Directory, "console.address")
				if err != nil {
					return fmt.Errorf("failed to read config string: %w", err)
				}

				// try with 100ms delays until we exceed the grace period
				apiKey, start := "", time.Now()
				for apiKey == "" {
					apiKey, err = newConsoleEndpoints(consoleAddress).createOrGetAPIKey(context.Background())
					if err != nil && time.Since(start) > gatewayGracePeriod {
						return fmt.Errorf("failed to create account: %w", err)
					}
					time.Sleep(100 * time.Millisecond)
				}

				satNodeID, err := identity.NodeIDFromCertPath(filepath.Join(satellite.Directory, "identity.cert"))
				if err != nil {
					return fmt.Errorf("failed to get node id from path: %w", err)
				}
				nodeURL := storj.NodeURL{
					ID:      satNodeID,
					Address: satellite.Address,
				}

				access, err := uplink.RequestAccessWithPassphrase(context.Background(), nodeURL.String(), apiKey, "")
				if err != nil {
					return fmt.Errorf("failed to get passphrase: %w", err)
				}

				accessData, err := access.Serialize()
				if err != nil {
					return fmt.Errorf("failed to serialize access: %w", err)
				}
				vip.Set("access", accessData)

				if err := vip.WriteConfig(); err != nil {
					return fmt.Errorf("failed to write config: %w", err)
				}
			}

			if runAccessData := vip.GetString("access"); runAccessData != accessData {
				process.AddExtra("ACCESS", runAccessData)

				if apiKey, err := getAPIKey(runAccessData); err == nil {
					process.AddExtra("API_KEY", apiKey)
				}
			}

			process.AddExtra("ACCESS_KEY", vip.GetString("minio.access-key"))
			process.AddExtra("SECRET_KEY", vip.GetString("minio.secret-key"))

			return nil
		}
	}

	// Create storage nodes
	if flags.StorageNodeCount > maxStoragenodeCount {
		return nil, fmt.Errorf("exceeded the max instance count of %d with Storage Node count of %d", maxStoragenodeCount, flags.StorageNodeCount)
	}
	for i := 0; i < flags.StorageNodeCount; i++ {
		process := processes.New(Info{
			Name:       fmt.Sprintf("storagenode/%d", i),
			Executable: "storagenode",
			Directory:  filepath.Join(processes.Directory, "storagenode", strconv.Itoa(i)),
			Address:    net.JoinHostPort(host, port(storagenodePeer, i, publicRPC)),
		})

		for _, satellite := range satellites {
			process.WaitForStart(satellite)
		}

		process.Arguments = withCommon(process.Directory, Arguments{
			"setup": {
				"--identity-dir", process.Directory,
				"--console.address", net.JoinHostPort(host, port(storagenodePeer, i, publicHTTP)),
				"--console.static-dir", filepath.Join(storjRoot, "web/storagenode/"),
				"--server.address", process.Address,
				"--server.private-address", net.JoinHostPort(host, port(storagenodePeer, i, privateRPC)),

				"--operator.email", fmt.Sprintf("storage%d@mail.test", i),
				"--operator.wallet", "0x0123456789012345678901234567890123456789",

				"--storage2.monitor.minimum-disk-space", "0",

				"--server.extensions.revocation=false",
				"--server.use-peer-ca-whitelist=false",

				"--version.server-address", fmt.Sprintf("http://%s/", versioncontrol.Address),
				"--debug.addr", net.JoinHostPort(host, port(storagenodePeer, i, debugHTTP)),

				"--tracing.app", fmt.Sprintf("storagenode/%d", i),
			},
			"run": {},
		})

		process.ExecBefore["setup"] = func(process *Process) error {
			whitelisted := []string{}
			for _, satellite := range satellites {
				peer, err := identity.PeerConfig{
					CertPath: filepath.Join(satellite.Directory, "identity.cert"),
				}.Load()
				if err != nil {
					return err
				}

				whitelisted = append(whitelisted, peer.ID.String()+"@"+satellite.Address)
			}

			process.Arguments["setup"] = append(process.Arguments["setup"],
				"--storage2.trust.sources", strings.Join(whitelisted, ","),
			)
			return nil
		}

		process.ExecBefore["run"] = func(process *Process) error {
			return readConfigString(&process.Address, process.Directory, "server.address")
		}
	}

	{ // setup multinode
		process := processes.New(Info{
			Name:       fmt.Sprintf("multinode/%d", 0),
			Executable: "multinode",
			Directory:  filepath.Join(processes.Directory, "multinode", strconv.Itoa(0)),
		})

		process.Arguments = withCommon(process.Directory, Arguments{
			"setup": {
				"--console.address", net.JoinHostPort(host, port(multinodePeer, 0, publicHTTP)),
				"--console.static-dir", filepath.Join(storjRoot, "web/multinode/"),
				"--debug.addr", net.JoinHostPort(host, port(multinodePeer, 0, debugHTTP)),
			},
			"run": {},
		})

		process.AddExtra("SETUP_ARGS", strings.Join(process.Arguments["setup"], " "))
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

func identitySetup(network *Processes) (*Processes, error) {
	processes := NewProcesses(network.Directory, network.FailFast)

	for _, process := range network.List {
		if process.Info.Executable == "gateway" || process.Info.Executable == "redis-server" {
			// gateways and redis-servers don't need an identity
			continue
		}

		if strings.Contains(process.Name, "satellite-") {
			// we only need to create the identity once for the satellite system, we create the
			// identity for the satellite process and share it with these other satellite processes
			continue
		}

		identity := processes.New(Info{
			Name:       "identity/" + process.Info.Name,
			Executable: "identity",
			Directory:  process.Directory,
			Address:    "",
		})

		identity.Arguments = Arguments{
			"setup": {
				"--identity-dir", process.Directory,
				"--concurrency", "1",
				"--difficulty", "8",
				"create", ".",
			},
		}
	}

	// create directories for all processes
	for _, process := range processes.List {
		if err := os.MkdirAll(process.Directory, folderPermissions); err != nil {
			return nil, err
		}
	}

	return processes, nil
}

// getAPIKey parses an access string to return its corresponding api key.
func getAPIKey(access string) (apiKey string, err error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return "", errors.New("invalid access grant format")
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return "", err
	}

	apiKey = base58.CheckEncode(p.ApiKey, 0)
	return apiKey, nil
}

// readConfigString reads from dir/config.yaml flagName returns the value in `into`.
func readConfigString(into *string, dir, flagName string) error {
	vip := viper.New()
	vip.AddConfigPath(dir)
	if err := vip.ReadInConfig(); err != nil {
		return err
	}
	if v := vip.GetString(flagName); v != "" {
		*into = v
	}
	return nil
}

// namespacedDatabaseURL returns an equivalent database url with the given namespace
// so that a database opened with the url does not conflict with other databases
// opened with a different namespace.
func namespacedDatabaseURL(dbURL, namespace string) (string, error) {
	parsed, err := url.Parse(dbURL)
	if err != nil {
		return "", err
	}

	switch dbutil.ImplementationForScheme(parsed.Scheme) {
	case dbutil.Postgres:
		return pgutil.ConnstrWithSchema(dbURL, namespace), nil

	case dbutil.Cockroach:
		parsed.Path += "/" + namespace
		return parsed.String(), nil

	default:
		return "", errs.New("unable to namespace db url: %q", dbURL)
	}
}
