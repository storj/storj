package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcmd"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
)

var (
	defaultDirs = flag.Bool("default-dirs", false, "run tests which execure commands that read/write files in the default directories")
	prebuiltTestCmds = flag.Bool("prebuild-test-cmds", false, "run tests using pre-built cli command binaries")
)

func TestCmdNewCA(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cmdIdentity, _ := testCommands(ctx, t)

	t.Run("with default cert & key paths", func(t *testing.T) {
		if !*defaultDirs {
			t.SkipNow()
		}

		assert := assert.New(t)

		err := cmdIdentity.Run("ca", "new")
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		assert.Equal(identity.CertKey.String(), newCACfg.CA.Status().String())

		defaultCertPath := filepath.Join(defaultConfDir, "ca.cert")
		_, err = os.Stat(defaultCertPath)
		assert.NoError(err)

		defaultKeyPath := filepath.Join(defaultConfDir, "ca.key")
		_, err = os.Stat(defaultKeyPath)
		assert.NoError(err)
	})

	t.Run("with difficulty", func(t *testing.T) {
		assert := assert.New(t)

		expectedDifficulty := 4
		err := cmdIdentity.Run(
			"ca", "new",
			"--ca.difficulty", strconv.Itoa(expectedDifficulty),
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		assert.Equal(identity.CertKey.String(), newCACfg.CA.Status().String())

		_, err = os.Stat(newCACfg.CA.CertPath)
		assert.NoError(err)

		_, err = os.Stat(newCACfg.CA.KeyPath)
		assert.NoError(err)

		ca, err := newCACfg.CA.FullConfig().Load()
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		difficulty, err := ca.ID.Difficulty()
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		assert.Condition(func() bool {
			return uint16(expectedDifficulty) <= difficulty
		})
	})

	t.Run("with non-default cert & key paths", func(t *testing.T) {
		assert := assert.New(t)
		credsPath := ctx.Dir("identity")
		certPath := filepath.Join(credsPath, "ca-non-default.cert")
		keyPath := filepath.Join(credsPath, "ca-non-default.key")

		err := cmdIdentity.Run(
			"ca", "new",
			"--ca.cert-path", certPath,
			"--ca.key-path", keyPath,
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		assert.Equal(identity.CertKey.String(), newCACfg.CA.Status().String())
		assert.Equal(certPath, certPath)
		assert.Equal(keyPath, keyPath)

		_, err = os.Stat(certPath)
		assert.NoError(err)

		_, err = os.Stat(keyPath)
		assert.NoError(err)
	})

	t.Run("with parent cert & key paths", func(t *testing.T) {
		// TODO: looks like this is broken; fix
		t.SkipNow()

		assert := assert.New(t)
		parentCertPath := ctx.File("parent.cert")
		parentKeyPath := ctx.File("parent.key")
		certPath := ctx.File("ca.cert")
		keyPath := ctx.File("ca.key")

		err := cmdIdentity.Run(
			"ca", "new",
			"--ca.parent-cert-path", parentCertPath,
			"--ca.parent-key-path", parentKeyPath,
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		_, err = os.Stat(certPath)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		_, err = os.Stat(keyPath)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		err = cmdIdentity.Run(
			"ca", "new",
			"--parent.cert-path", parentCertPath,
			"--parent.key-path", parentKeyPath,
			"--ca.cert-path", certPath,
			"--ca.key-path", keyPath,
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		assert.Equal(identity.CertKey.String(), newCACfg.CA.Status().String())

		_, err = os.Stat(certPath)
		assert.NoError(err)

		_, err = os.Stat(keyPath)
		assert.NoError(err)

		ca, err := identity.FullCAConfig{
			CertPath: certPath,
			KeyPath:  keyPath,
		}.Load()
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		assert.NotEmpty(ca.RestChain)
	})
}

// TODO: move to main_test.go
// TODO: test cmdNewService with no args?
func TestCmdNewService(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cmdIdentity, _ := testCommands(ctx, t)

	assert := assert.New(t)

	services := []string{
		"storagenode",
		"uplink",
		"satellite",
		"certificates",
	}

	t.Run("with default base-dir", func(t *testing.T) {
		if !*defaultDirs {
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

				err := cmdIdentity.Run(
					"new", service,
					"--base-dir", baseDir,
				)
				if !assert.NoError(err) {
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

	assert := assert.New(t)

	t.Run("with default base-dir", func(t *testing.T) {
		if !*defaultDirs {
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

		// Start CSR service
		err := cmdCertificates.Start(
			append([]string{
				"run",
				"--signer.min-difficulty", "0",
				"--server.address", csrAddress,
			}, certificatesFlags...)...,
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}
		certsServerProcess := *cmdCertificates.Process
		defer ctx.Check(certsServerProcess.Kill)
		time.Sleep(1 * time.Second)

		// Create signer cert
		err = cmdIdentity.Run(
			"ca", "new",
			"--ca.cert-path", signerCertPath,
			"--ca.key-path", signerKeyPath,
		)

		// Sign identity
		err = cmdIdentity.Run(
			"csr", service,
			"--base-dir", baseDir,
			"--signer.address", csrAddress,
			"--signer.auth-token", *token,
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		//Verify
		err = cmdCertificates.Run(
			"verify", service,
			"--config-dir", certsConfigDir,
			"--ca.cert-path", caCertPath,
			"--ca.key-path", caKeyPath,
			"--identity.cert-path", identCertPath,
			"--identity.key-path", identKeyPath,
		)
		if !assert.NoError(err) {
			t.Fatal(err)
		}

		// TODO: ensure that auth is claimed
		ca, err := caConfig.FullConfig().Load()
		if assert.NoError(err) {
			assert.NotEmpty(ca.RestChain)
		}
		ident, err := identConfig.FullConfig().Load()
		if assert.NoError(err) {
			assert.NotEmpty(ident.RestChain)
		}
		assert.Equal(identity.CertKey.String(), caConfig.Status().String())
		assert.Equal(identity.CertKey.String(), identConfig.Status().String())
	})
}

func testCommands(ctx *testcontext.Context, t *testing.T) (_, _ *testcmd.Cmd) {
	if *prebuiltTestCmds {
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
	tokenParts := strings.SplitN(stdout.String(), ",", 2)
	*token = tokenParts[1][:len(tokenParts[1])-1]
}
