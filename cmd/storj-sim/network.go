// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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

	"storj.io/common/fpath"
	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/processgroup"
)

const (
	maxInstanceCount    = 100
	maxStoragenodeCount = 200

	folderPermissions = 0744
)

var (
	defaultAPIKeyData = "13YqgH45XZLg7nm6KsQ72QgXfjbDu2uhTaeSdMVP2A85QuANthM9K58ww5Y4nhMowrZDoqdA4Kyqt1ioQghQcm9fT5uR2drPHpFEqeb"
	defaultAPIKey, _  = uplink.ParseAPIKey(defaultAPIKeyData)
)

const (
	// The following values of peer class and endpoints are used
	// to create a port with a consistent format for storj-sim services.

	// Peer class
	satellitePeer      = 0
	gatewayPeer        = 1
	versioncontrolPeer = 2
	storagenodePeer    = 3

	// Endpoint
	publicGRPC  = 0
	privateGRPC = 1
	publicHTTP  = 2
	privateHTTP = 3
	debugHTTP   = 9

	// satellite specific constants
	redisPort          = 4
	debugMigrationHTTP = 6
	debugPeerHTTP      = 7
	debugRepairerHTTP  = 8
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

	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	if command == "setup" {
		if flags.Postgres == "" {
			return errors.New("postgres connection URL is required for running storj-sim. Example: `storj-sim network setup --postgres=<connection URL>`.\nSee docs for more details https://github.com/storj/docs/blob/master/Test-network.md#running-tests-with-postgres")
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

	ctx, cancel := NewCLIContext(context.Background())

	var group errgroup.Group
	processes.Start(ctx, &group, "run")

	for _, process := range processes.List {
		process.Status.Started.Wait(ctx)
	}
	if err := ctx.Err(); err != nil {
		return err
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
		}
		for command, args := range all {
			all[command] = append(append(common, command), args...)
		}
		return all
	}

	processes := NewProcesses(flags.Directory)

	var host = flags.Host
	versioncontrol := processes.New(Info{
		Name:       "versioncontrol/0",
		Executable: "versioncontrol",
		Directory:  filepath.Join(processes.Directory, "versioncontrol", "0"),
		Address:    net.JoinHostPort(host, port(versioncontrolPeer, 0, publicGRPC)),
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
				Directory:  filepath.Join(processes.Directory, "satellite", fmt.Sprint(i), "redis"),
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
				err := ioutil.WriteFile(confpath, []byte(conf), 0755)
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
			Directory:  filepath.Join(processes.Directory, "satellite", fmt.Sprint(i)),
			Address:    net.JoinHostPort(host, port(satellitePeer, i, publicGRPC)),
		})
		satellites = append(satellites, apiProcess)

		consoleAuthToken := "secure_token"

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
				// TODO: remove console.auth-token after vanguard release
				"--console.auth-token", consoleAuthToken,
				"--marketing.base-url", "",
				"--marketing.address", net.JoinHostPort(host, port(satellitePeer, i, privateHTTP)),
				"--marketing.static-dir", filepath.Join(storjRoot, "web/marketing/"),
				"--server.address", apiProcess.Address,
				"--server.private-address", net.JoinHostPort(host, port(satellitePeer, i, privateGRPC)),

				"--live-accounting.storage-backend", "redis://" + redisAddress + "?db=" + strconv.Itoa(redisPortBase),
				"--server.revocation-dburl", "redis://" + redisAddress + "?db=" + strconv.Itoa(redisPortBase+1),

				"--server.extensions.revocation=false",
				"--server.use-peer-ca-whitelist=false",

				"--mail.smtp-server-address", "smtp.gmail.com:587",
				"--mail.from", "Storj <yaroslav-satellite-test@storj.io>",
				"--mail.template-path", filepath.Join(storjRoot, "web/satellite/static/emails"),
				"--version.server-address", fmt.Sprintf("http://%s/", versioncontrol.Address),
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugHTTP)),
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
			)
		}
		apiProcess.ExecBefore["run"] = func(process *Process) error {
			return readConfigString(&process.Address, process.Directory, "server.address")
		}

		migrationProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-migration/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", fmt.Sprint(i)),
		})
		migrationProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"migration",
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugMigrationHTTP)),
			},
		})

		coreProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-core/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", fmt.Sprint(i)),
			Address:    "",
		})
		coreProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugPeerHTTP)),
			},
		})
		coreProcess.WaitForExited(migrationProcess)

		repairProcess := processes.New(Info{
			Name:       fmt.Sprintf("satellite-repairer/%d", i),
			Executable: "satellite",
			Directory:  filepath.Join(processes.Directory, "satellite", fmt.Sprint(i)),
		})
		repairProcess.Arguments = withCommon(apiProcess.Directory, Arguments{
			"run": {
				"repair",
				"--debug.addr", net.JoinHostPort(host, port(satellitePeer, i, debugRepairerHTTP)),
			},
		})
		repairProcess.WaitForExited(migrationProcess)

		apiProcess.WaitForExited(migrationProcess)
	}

	// Create gateways for each satellite
	for i, satellite := range satellites {
		satellite := satellite
		process := processes.New(Info{
			Name:       fmt.Sprintf("gateway/%d", i),
			Executable: "gateway",
			Directory:  filepath.Join(processes.Directory, "gateway", fmt.Sprint(i)),
			Address:    net.JoinHostPort(host, port(gatewayPeer, i, publicGRPC)),
		})

		scopeData, err := (&uplink.Scope{
			SatelliteAddr:    satellite.Address,
			APIKey:           defaultAPIKey,
			EncryptionAccess: uplink.NewEncryptionAccessWithDefaultKey(storj.Key{}),
		}).Serialize()
		if err != nil {
			return nil, err
		}

		// gateway must wait for the corresponding satellite to start up
		process.WaitForStart(satellite)
		process.Arguments = withCommon(process.Directory, Arguments{
			"setup": {
				"--non-interactive",

				"--scope", scopeData,
				"--identity-dir", process.Directory,
				"--server.address", process.Address,

				"--rs.min-threshold", strconv.Itoa(1 * flags.StorageNodeCount / 5),
				"--rs.repair-threshold", strconv.Itoa(2 * flags.StorageNodeCount / 5),
				"--rs.success-threshold", strconv.Itoa(3 * flags.StorageNodeCount / 5),
				"--rs.max-threshold", strconv.Itoa(4 * flags.StorageNodeCount / 5),

				"--tls.extensions.revocation=false",
				"--tls.use-peer-ca-whitelist=false",

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
			if runScopeData := vip.GetString("scope"); !flags.OnlyEnv && runScopeData == scopeData {
				var consoleAddress string
				err := readConfigString(&consoleAddress, satellite.Directory, "console.address")
				if err != nil {
					return err
				}

				// try with 100ms delays until we hit 3s
				apiKey, start := "", time.Now()
				for apiKey == "" {
					apiKey, err = newConsoleEndpoints(consoleAddress).createOrGetAPIKey()
					if err != nil && time.Since(start) > 3*time.Second {
						return err
					}
					time.Sleep(100 * time.Millisecond)
				}

				scope, err := uplink.ParseScope(runScopeData)
				if err != nil {
					return err
				}
				scope.APIKey, err = uplink.ParseAPIKey(apiKey)
				if err != nil {
					return err
				}
				scopeData, err := scope.Serialize()
				if err != nil {
					return err
				}
				vip.Set("scope", scopeData)

				if err := vip.WriteConfig(); err != nil {
					return err
				}
			}

			if runScopeData := vip.GetString("scope"); runScopeData != scopeData {
				process.AddExtra("SCOPE", runScopeData)
				if scope, err := uplink.ParseScope(runScopeData); err == nil {
					process.AddExtra("API_KEY", scope.APIKey.Serialize())
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
			Directory:  filepath.Join(processes.Directory, "storagenode", fmt.Sprint(i)),
			Address:    net.JoinHostPort(host, port(storagenodePeer, i, publicGRPC)),
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
				"--server.private-address", net.JoinHostPort(host, port(storagenodePeer, i, privateGRPC)),

				"--operator.email", fmt.Sprintf("storage%d@mail.test", i),
				"--operator.wallet", "0x0123456789012345678901234567890123456789",

				"--storage2.monitor.minimum-disk-space", "0",
				"--storage2.monitor.minimum-bandwidth", "0",

				"--server.extensions.revocation=false",
				"--server.use-peer-ca-whitelist=false",

				"--version.server-address", fmt.Sprintf("http://%s/", versioncontrol.Address),
				"--debug.addr", net.JoinHostPort(host, port(storagenodePeer, i, debugHTTP)),
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
	processes := NewProcesses(network.Directory)

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

// readConfigString reads from dir/config.yaml flagName returns the value in `into`
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
