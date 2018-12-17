package certfiicates

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

var (
	rootCmd = &cobra.Command{
		Use:   "certificates",
		Short: "Certificate request signing",
	}

	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Setup a certificate signing server",
		RunE:  cmdSetup,
	}

	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run a certificate signing server",
		RunE:  cmdRun,
	}

	authCmd = &cobra.Command{
		Use:   "auth",
		Short: "CSR authorization management",
	}

	authAddCmd = &cobra.Command{
		Use: "add",
		Short: "Add authorizations from a list of emails",
	}

	setupCfg struct {
		Overwrite bool
		// NB: cert and key paths overridden in setup
		CA provider.CASetupConfig
		// NB: cert and key paths overridden in setup
		Identity provider.IdentitySetupConfig
	}

	runCfg struct {
		CA       provider.FullCAConfig
		Identity provider.IdentityConfig
		CertSigner certificates.CertSignerConfig
	}

	defaultConfDir = fpath.ApplicationDir("storj", "cert-signing")
	confDir        = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for certificate request signing configuration")
)

func init() {
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg)
	rootCmd.AddCommand(authCmd)
}

func cmdSetup(cmd *cobra.Command, args []string) error {
	setupDir, err := filepath.Abs(*confDir)
	if err != nil {
		return err
	}

	valid, err := fpath.IsValidSetupDir(setupDir)
	if !setupCfg.Overwrite && !valid {
		fmt.Printf("certificate signer configuration already exists (%v). rerun with --overwrite\n", setupDir)
		return nil
	}

	err = os.MkdirAll(setupDir, 0700)
	if err != nil {
		return err
	}

	setupCfg.CA.CertPath = filepath.Join(setupDir, "ca.cert")
	setupCfg.CA.KeyPath = filepath.Join(setupDir, "ca.key")
	setupCfg.Identity.CertPath = filepath.Join(setupDir, "identity.cert")
	setupCfg.Identity.KeyPath = filepath.Join(setupDir, "identity.key")

	err = provider.SetupCA(process.Ctx(cmd), setupCfg.CA)
	if err != nil {
		return err
	}

	o := map[string]interface{}{
		"ca.cert-path":       setupCfg.CA.CertPath,
		"ca.key-path":        setupCfg.CA.KeyPath,
		"identity.cert-path": setupCfg.Identity.CertPath,
		"identity.key-path":  setupCfg.Identity.KeyPath,
	}
	return process.SaveConfig(runCmd.Flags(),
		filepath.Join(setupDir, "config.yaml"), o)
}

func cmdRun(cmd *cobra.Command, args []string) error {
	ctx := process.Ctx(cmd)

	return runCfg.Identity.Run(ctx, nil, runCfg.CertSigner)
}

func main() {
	process.Exec(rootCmd)
}
