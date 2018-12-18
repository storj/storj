package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/utils"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/provider"
)

type batchCfg struct {
	EmailsPath string `help:"optional path to a list of emails, delimited by <delimiter>, for batch processing"`
	Delimiter  string `help:"delimiter to split emails loaded from <emails-path> on (e.g. comma, new-line)" default:"\n"`
}

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
		Use:   "add <auth_increment_count> [<email>, ...]",
		Short: "Create authorizations from a list of emails",
		Args:  cobra.MinimumNArgs(1),
		RunE:  cmdAddAuth,
	}

	authGetCmd = &cobra.Command{
		Use:   "get [<email>, ...]",
		Short: "Get authorization(s) info from CSR authorization DB",
		RunE:  cmdGetAuth,
	}

	setupCfg struct {
		Overwrite bool
		// NB: cert and key paths overridden in setup
		CA provider.CASetupConfig
		// NB: cert and key paths overridden in setup
		Identity provider.IdentitySetupConfig
	}

	runCfg struct {
		CertSigner certificates.CertSignerConfig
		CA         provider.FullCAConfig
		Identity   provider.IdentityConfig
	}

	authAddCfg struct {
		certificates.CertSignerConfig
		batchCfg
	}

	authGetCfg struct {
		certificates.CertSignerConfig
		batchCfg
	}

	defaultConfDir = fpath.ApplicationDir("storj", "cert-signing")
	confDir        = rootCmd.PersistentFlags().String("config-dir", defaultConfDir, "main directory for certificate request signing configuration")
)

func init() {
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg)
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authAddCmd)
	cfgstruct.Bind(authAddCmd.Flags(), &authAddCfg)
	authCmd.AddCommand(authGetCmd)
	cfgstruct.Bind(authGetCmd.Flags(), &authGetCfg)
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

func cmdAddAuth(cmd *cobra.Command, args []string) error {
	count := args[0]
	authDB, err := authAddCfg.NewAuthDB()
	if err != nil {
		return err
	}

	var emails []string
	if len(args) > 1 {
		if authAddCfg.EmailsPath != "" {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args[1:]
	} else {
		list, err := ioutil.ReadFile(authAddCfg.EmailsPath)
		if err != nil {
			return errs.Wrap(err)
		}
		emails = strings.Split(string(list), authAddCfg.Delimiter)
	}

	var incErrs utils.ErrorGroup
	for _, email := range emails {
		if err := authDB.Create(email, count); err != nil {
			incErrs.Add(err)
		}
	}
	return incErrs.Finish()
}

func cmdGetAuth(cmd *cobra.Command, args []string) error {
	authDB, err := authGetCfg.NewAuthDB()
	if err != nil {
		return err
	}

	var emails []string
	if len(args) > 1 {
		if authAddCfg.EmailsPath != "" {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args[1:]
	} else {
		list, err := ioutil.ReadFile(authAddCfg.EmailsPath)
		if err != nil {
			return errs.Wrap(err)
		}
		emails = strings.Split(string(list), authAddCfg.Delimiter)
	}

	var emailErrs, printErrs utils.ErrorGroup
	w := tabwriter.NewWriter(os.Stdout, 0, 1, 1, ' ', 0)
	if _, err := fmt.Fprintln(w, "Email\tClaimed\tAvail.\t"); err != nil {
		return err
	}

	for _, email := range emails {
		auths, err := authDB.Get(email)
		if err != nil {
			emailErrs.Add(err)
			continue
		}


		if _, err := fmt.Fprintf(w,
			"%s\t%d\t%d\t\n",
			email,
			len(auths.claimed()),
			len(auths.avail()),
		); err != nil {
			printErrs.Add(err)
		}
	}

	return utils.CombineErrors(emailErrs.Finish(), printErrs.Finish())
}

func main() {
	process.Exec(rootCmd)
}
