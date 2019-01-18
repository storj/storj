// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os/exec"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/provider"
)

type newCACfg struct {
	CA provider.CASetupConfig
}

func TestCA(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	identityexe := ctx.Compile("storj.io/storj/cmd/identity")
	// certificatesexe := ctx.Compile("storj.io/cmd/certificates")

	t.Run("default", func(t *testing.T) {
		output, err := exec.Command(identityexe,
			"--config-dir", ctx.Dir("basic"),
			"--ca.difficulty", "4",
			"ca", "new", "main",
		).CombinedOutput()
		t.Log(string(output))
		require.NoError(t, err)

		caConfig := identity.SetupConfig{
			CertPath: ctx.File("basic", "main", "ca.cert"),
			KeyPath:  ctx.File("basic", "main", "ca.key"),
		}
		assert.Equal(t, caConfig.Status(), identity.CertKey)
	})

	t.Run("custom paths", func(t *testing.T) {
		const expectedDifficulty = 4

		output, err := exec.Command(identityexe,
			"--config-dir", ctx.Dir("difficulty"),
			"--ca.difficulty", strconv.Itoa(expectedDifficulty),

			"--ca.cert-path", ctx.File("difficulty", "custom.ca.cert"),
			"--ca.key-path", ctx.File("difficulty", "custom.ca.key"),

			"ca", "new", "main",
		).CombinedOutput()
		t.Log(string(output))
		require.NoError(t, err)

		caConfig := identity.SetupConfig{
			CertPath: ctx.File("difficulty", "custom.identity.cert"),
			KeyPath:  ctx.File("difficulty", "custom.identity.key"),
		}
		assert.Equal(t, caConfig.Status(), identity.CertKey)

		ca, err := caConfig.FullConfig().Load()
		require.NoError(t, err)

		caDifficulty, err := ca.ID.Difficulty()
		require.NoError(t, err)

		assert.Condition(t, func() bool {
			return uint16(expectedDifficulty) <= caDifficulty
		})
	})

	t.Run("with parent", func(t *testing.T) {
		output, err := exec.Command(identityexe,
			"--config-dir", ctx.Dir("basic"),
			"--ca.difficulty", "4",
			"ca", "new", "main",
		).CombinedOutput()
		t.Log(string(output))
		require.NoError(t, err)

		caConfig := identity.SetupConfig{
			CertPath: ctx.File("basic", "main", "ca.cert"),
			KeyPath:  ctx.File("basic", "main", "ca.key"),
		}
		assert.Equal(t, caConfig.Status(), identity.CertKey)
	})

}

func TestNew(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	identityexe := ctx.Compile("storj.io/storj/cmd/identity")
	// certificatesexe := ctx.Compile("storj.io/cmd/certificates")

	t.Run("default", func(t *testing.T) {
		const expectedDifficulty = 4

		output, err := exec.Command(identityexe,
			"--config-dir", ctx.Dir("basic"),
			"--difficulty", strconv.Itoa(expectedDifficulty),
			"new", "main",
		).CombinedOutput()
		t.Log(string(output))
		require.NoError(t, err)

		caConfig := identity.CASetupConfig{
			CertPath: ctx.File("basic", "main", "ca.cert"),
			KeyPath:  ctx.File("basic", "main", "ca.key"),
		}
		assert.Equal(t, caConfig.Status(), identity.CertKey)

		identityConfig := identity.SetupConfig{
			CertPath: ctx.File("basic", "main", "identity.cert"),
			KeyPath:  ctx.File("basic", "main", "identity.key"),
		}
		assert.Equal(t, identityConfig.Status(), identity.CertKey)

		ca, err := caConfig.FullConfig().Load()
		require.NoError(t, err)

		caDifficulty, err := ca.ID.Difficulty()
		require.NoError(t, err)

		assert.Condition(t, func() bool {
			return uint16(expectedDifficulty) <= caDifficulty
		})

		identity, err := identityConfig.FullConfig().Load()
		require.NoError(t, err)

		identityDifficulty, err := identity.ID.Difficulty()
		require.NoError(t, err)

		assert.Condition(t, func() bool {
			return uint16(expectedDifficulty) <= identityDifficulty
		})
	})
}

/*

	t.Run("with parent cert & key paths", func(t *testing.T) {
		// TODO: looks like this is broken; fix
		t.SkipNow()

		parentCertPath := ctx.File("parent.cert")
		parentKeyPath := ctx.File("parent.key")
		certPath := ctx.File("ca.cert")
		keyPath := ctx.File("ca.key")

		_, err := exec.Command(
			identity,
			"ca", "new",
			"--ca.parent-cert-path", parentCertPath,
			"--ca.parent-key-path", parentKeyPath,
		).CombinedOutput()
		require.NoError(t, err)

		_, err = os.Stat(certPath)
		require.NoError(t, err)

		_, err = os.Stat(keyPath)
		require.NoError(t, err)

		_, err = exec.Command(identityexe,
			"ca", "new",
			"--parent.cert-path", parentCertPath,
			"--parent.key-path", parentKeyPath,
			"--ca.cert-path", certPath,
			"--ca.key-path", keyPath,
		).CombinedOutput()
		require.NoError(t, err)

		assert.Equal(t, identity.CertKey.String(), newCACfg.CA.Status().String())

		_, err = os.Stat(certPath)
		assert.NoError(t, err)

		_, err = os.Stat(keyPath)
		assert.NoError(t, err)

		ca, err := identity.FullCAConfig{
			CertPath: certPath,
			KeyPath:  keyPath,
		}.Load()
		require.NoError(t, err)

		assert.NotEmpty(t, ca.RestChain)
	})
}

/*
// TODO: move to main_test.go
// TODO: test cmdNewService with no args?
func TestCmdNewService(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cmdIdentity, _ := testCommands(ctx, t)
	services := []string{
		"storagenode",
		"uplink",
		"satellite",
		"certificates",
	}

	t.Run("with default base-dir", func(t *testing.T) {
		if !*testcmd.DefaultDirs {
			t.SkipNow()
		}

		// TODO: test this
		t.FailNow()
	})

	t.Run("with non-default base-dir", func(t *testing.T) {
		baseDir := ctx.Dir("identity")

		for _, service := range services[:1] {
			t.Run("service: "+service, func(t *testing.T) {
				servicePath := ctx.Dir("identity", service)
				caConfig := identity.CASetupConfig{
					CertPath: filepath.Join(servicePath, "ca.cert"),
					KeyPath:  filepath.Join(servicePath, "ca.key"),
				}
				identConfig := identity.SetupConfig{
					CertPath: filepath.Join(servicePath, "identity.cert"),
					KeyPath:  filepath.Join(servicePath, "identity.key"),
				}

				if !assert.Equal(identity.NoCertNoKey.String(), caConfig.Status().String()) {
					t.Fatal(errs.New("expected ca to not exist for service: %s", service))
				}
				if !assert.Equal(identity.NoCertNoKey.String(), identConfig.Status().String()) {
					t.Fatal(errs.New("expected ca to not exist for service: %s", service))
				}

				err := exec.Command(identityexe,
					"new", service,
					"--base-dir", baseDir,
				)
				if !assert.NoError(t, err) {
					t.Fatal(err)
				}

				assert.Equal(identity.CertKey.String(), caConfig.Status().String())
				assert.Equal(identity.CertKey.String(), identConfig.Status().String())
			})
		}
	})
}

// TODO: move to main_test.go
func TestCmdSigningRequest(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cmdIdentity, cmdCertificates := testCommands(ctx, t)


	t.Run("with default base-dir", func(t *testing.T) {
		if !*testcmd.DefaultDirs {
			t.SkipNow()
		}

		// TODO: test this
		t.FailNow()
	})

	t.Run("with non-default base-dir", func(t *testing.T) {
		csrAddress := "127.0.0.1:8888"
		baseDir := ctx.Dir("identity")
		service := "storagenode"
		servicePath := ctx.Dir("identity", service)
		caCertPath := filepath.Join(servicePath, "ca.cert")
		caKeyPath := filepath.Join(servicePath, "ca.key")
		identCertPath := filepath.Join(servicePath, "identity.cert")
		identKeyPath := filepath.Join(servicePath, "identity.key")
		caConfig := identity.CASetupConfig{
			CertPath: caCertPath,
			KeyPath:  caKeyPath,
		}
		identConfig := identity.SetupConfig{
			CertPath: identCertPath,
			KeyPath:  identKeyPath,
		}

		if !assert.Equal(identity.NoCertNoKey.String(), caConfig.Status().String()) {
			t.Fatal(errs.New("expected ca to not exist for service: %s", service))
		}
		if !assert.Equal(identity.NoCertNoKey.String(), identConfig.Status().String()) {
			t.Fatal(errs.New("expected ca to not exist for service: %s", service))
		}

		token := new(string)
		certsConfigDir := ctx.Dir("certificates")
		signerCertPath := ctx.File("certificates", "signer.cert")
		signerKeyPath := ctx.File("certificates", "signer.key")
		certificatesFlags := []string{
			"--config-dir", certsConfigDir,
		}

		// Prepare authorizations
		setupCertificates(t, cmdCertificates, certificatesFlags)
		createAuthorization(t, cmdCertificates, certificatesFlags, token)

		// Create identity
		testidentity.NewTestIdentityFromCmd(t, cmdIdentity, caConfig.FullConfig(), identConfig.FullConfig())

		// Create signer cert
		err := exec.Command(identityexe,
			"ca", "new",
			"--ca.cert-path", signerCertPath,
			"--ca.key-path", signerKeyPath,
		)
		require.NoError(t, err)

		// Start CSR service
		err = cmdCertificates.Start(
			append([]string{
				"run",
				"--signer.min-difficulty", "0",
				"--server.address", csrAddress,
			}, certificatesFlags...)...,
		)
		require.NoError(t, err)
		time.Sleep(1 * time.Second)

		// Sign identity
		err = exec.Command(identityexe,
			"csr", service,
			"--base-dir", baseDir,
			"--signer.address", csrAddress,
			"--signer.auth-token", *token,
		)
		// TODO: need a more robust way to ensure CSR server is killed *always*
		ctx.Check(cmdCertificates.Kill)
		require.NoError(t, err)

		//Verify
		err = cmdCertificates.Run(
			"verify", service,
			"--config-dir", certsConfigDir,
			"--ca.cert-path", caCertPath,
			"--ca.key-path", caKeyPath,
			"--identity.cert-path", identCertPath,
			"--identity.key-path", identKeyPath,
		)
		require.NoError(t, err)

		// TODO: ensure that auth is claimed
		ca, err := caConfig.FullConfig().Load()
		if assert.NoError(t, err) {
			assert.NotEmpty(ca.RestChain)
		}
		ident, err := identConfig.FullConfig().Load()
		if assert.NoError(t, err) {
			assert.NotEmpty(ident.RestChain)
		}
		assert.Equal(identity.CertKey.String(), caConfig.Status().String())
		assert.Equal(identity.CertKey.String(), identConfig.Status().String())
	})
}

func testCommands(ctx *testcontext.Context, t *testing.T) (_, _ *testcmd.Cmd) {
	if *testcmd.PrebuiltTestCmds {
		return testcmd.NewCmd(testcmd.CmdIdentity.String()),
			testcmd.NewCmd(testcmd.CmdCertificates.String())
	}
	cmdMap, err := testcmd.Build(ctx, testcmd.CmdIdentity, testcmd.CmdCertificates)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	return cmdMap[testcmd.CmdIdentity], cmdMap[testcmd.CmdCertificates]
}

func setupCertificates(t *testing.T, cmdCertificates *testcmd.Cmd, flags []string) {
	err := cmdCertificates.Run(
		append([]string{
			"setup",
		}, flags...)...,
	)
	if !assert.NoError(t, err) {
		t.Fatal(errs.Combine(errs.New("certificates setup error"), err))
	}
}

func createAuthorization(t *testing.T, cmdCertificates *testcmd.Cmd, flags []string, token *string) {
	userID := "user@example.com"

	err := cmdCertificates.Run(
		append([]string{
			"auth", "create", "1", userID,
		}, flags...)...,
	)
	if !assert.NoError(t, err) {
		t.Fatal(errs.Combine(errs.New("certificates auth create error"), err))
	}

	err = cmdCertificates.DrainStdout()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = cmdCertificates.Run(
		append([]string{
			"auth", "export", userID,
		}, flags...)...,
	)
	if !assert.NoError(t, err) {
		t.Fatal(errs.Combine(errs.New("certificates auth create error"), err))
	}
	stdout, err := cmdCertificates.UnreadStdout()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	tokenParts := strings.SplitN(stdout.String(), ",", 2)
	*token = tokenParts[1][:len(tokenParts[1])-1]
}
*/
