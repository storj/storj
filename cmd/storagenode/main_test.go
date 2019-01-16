package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"testing"

	"storj.io/storj/internal/testidentity"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcmd"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
)

var (
	defaultDirs = flag.Bool("default-dirs", false, "run tests which exercise commands that read/write files in the default directories")
	prebuiltTestCmds = flag.Bool("prebuild-test-cmds", false, "run tests using pre-built cli command binaries")
)

func TestMain(m *testing.M) {
	flag.Parse()
	m.Run()
}

func TestCmdRun(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cmdStorageNode, cmdIdentity := testCommands(ctx, t)

	t.Run("with defaults", func(t *testing.T) {
		if !*defaultDirs {
			t.SkipNow()
		}

		// TODO: test this
		t.FailNow()
		// ...
	})

	t.Run("with non-default config-dir & credentials paths", func(t *testing.T) {
		configDir := ctx.Dir("config1")
		certPath := filepath.Join(configDir, "identity.cert")
		keyPath := filepath.Join(configDir, "identity.key")
		caConfig := identity.FullCAConfig{
			CertPath: filepath.Join(configDir, "ca.cert"),
			KeyPath:  filepath.Join(configDir, "ca.key"),
		}
		identConfig := identity.Config{
			CertPath: certPath,
			KeyPath:  keyPath,
		}

		// Storagenode setup
		err := cmdStorageNode.Run(
			"setup",
			"--config-dir", configDir,
			"--kademlia.operator.wallet", "0x6839992C7F5Bfbe7a3675C1d0aB06D33fcE084FA",
			"--kademlia.operator.email", "user@example.com",
		)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}

		t.Run("should error without existing, valid identity", func(t *testing.T) {
			// TODO: fix
			t.SkipNow()

			_, err := identity.Config{
				CertPath: certPath,
				KeyPath:  keyPath,
			}.Load()
			if !assert.True(t, peertls.ErrNotExist.Has(err)) {
				t.Fatal("expected identity to not exist")
			}

			err = cmdStorageNode.Run("--config-dir", configDir)
			assert.Error(t, err)
		})

		t.Run("should not error with valid, existing identity", func(t *testing.T) {
			testidentity.NewTestIdentityFromCmd(t, cmdIdentity, caConfig, identConfig)

			err = cmdStorageNode.Start("--config-dir", configDir)
			assert.NoError(t, err)
			ctx.Check(cmdStorageNode.Kill)
		})
	})

	t.Run("with non-default config-dir & default creds-dir", func(t *testing.T) {
		configDir := ctx.Dir("config2")
		certPath := filepath.Join(defaultCredsDir, "identity.cert")
		keyPath := filepath.Join(defaultCredsDir, "identity.key")

		// Storagenode setup
		err := cmdStorageNode.Run(
			"setup",
			"--config-dir", configDir,
			"--kademlia.operator.wallet", "0x6839992C7F5Bfbe7a3675C1d0aB06D33fcE084FA",
			"--kademlia.operator.email", "user@example.com",
		)
		fmt.Println(cmdStorageNode.Stderr)
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}

		t.Run("should error without existing, valid identity", func(t *testing.T) {
			// TODO: fix
			t.SkipNow()

			_, err := identity.Config{
				CertPath: certPath,
				KeyPath:  keyPath,
			}.Load()
			if !assert.True(t, peertls.ErrNotExist.Has(err)) {
				t.Fatal("expected identity to not exist")
			}

			err = cmdStorageNode.Run("--config-dir", configDir)
			assert.Error(t, err)
		})

		t.Run("should not error with valid, existing identity", func(t *testing.T) {
			// TODO: ensure credentials don't exist before creation

			// Create credentials (CA & identity files)
			err := cmdIdentity.Run(
				"new", "storagenode",
				"--creds-dir", credsDir,
			)
			if !assert.NoError(t, err) {
				t.Fatal(err)
			}

			// TODO: ensure credentials exist after creation

			err = cmdStorageNode.Start("--config-dir", configDir)
			assert.NoError(t, err)
			ctx.Check(cmdStorageNode.Kill)
		})
	})
}

func testCommands(ctx *testcontext.Context, t *testing.T) (_, _ *testcmd.Cmd) {
	if *prebuiltTestCmds {
		return testcmd.NewCmd(testcmd.CmdIdentity.String()),
			testcmd.NewCmd(testcmd.CmdCertificates.String())
	}
	cmdMap, err := testcmd.Build(ctx, testcmd.CmdIdentity, testcmd.CmdStorageNode)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	return cmdMap[testcmd.CmdStorageNode], cmdMap[testcmd.CmdIdentity]
}
