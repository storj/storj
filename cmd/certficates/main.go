package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/utils"

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

	authCreateCmd = &cobra.Command{
		Use:   "create <auth_increment_count> [<email>, ...]",
		Short: "Create authorizations from a list of emails",
		Args:  cobra.MinimumNArgs(1),
		RunE:  cmdCreateAuth,
	}

	authGetCmd = &cobra.Command{
		Use:   "get [<email>, ...]",
		Short: "Get authorization(s) info from CSR authorization DB",
		RunE:  cmdGetAuth,
	}

	setupCfg struct {
		Overwrite bool `default:"false"`
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

	authCreateCfg struct {
		certificates.CertSignerConfig
		batchCfg
	}

	authGetCfg struct {
		certificates.CertSignerConfig
		batchCfg
	}

	defaultConfDir = fpath.ApplicationDir("storj", "cert-signing")
)

func init() {
	rootCmd.AddCommand(setupCmd)
	cfgstruct.Bind(setupCmd.Flags(), &setupCfg, cfgstruct.ConfDir(defaultConfDir))
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authCreateCmd)
	cfgstruct.Bind(authCreateCmd.Flags(), &authCreateCfg, cfgstruct.ConfDir(defaultConfDir))
	authCmd.AddCommand(authGetCmd)
	cfgstruct.Bind(authGetCmd.Flags(), &authGetCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdSetup(cmd *cobra.Command, args []string) error {
	setupDir, err := filepath.Abs(defaultConfDir)
	if err != nil {
		return err
	}

	valid, err := fpath.IsValidSetupDir(setupDir)
	if err != nil {
		return err
	}
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

func cmdCreateAuth(cmd *cobra.Command, args []string) error {
	count, err := strconv.Atoi(args[0])
	if err != nil {
		return errs.New("Count couldn't be parsed: %s", args[0])
	}
	authDB, err := authCreateCfg.NewAuthDB()
	if err != nil {
		return err
	}

	var emails []string
	if len(args) > 1 {
		if authCreateCfg.EmailsPath != "" {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args[1:]
	} else {
		list, err := ioutil.ReadFile(authCreateCfg.EmailsPath)
		if err != nil {
			return errs.Wrap(err)
		}
		emails = strings.Split(string(list), authCreateCfg.Delimiter)
	}

	var incErrs utils.ErrorGroup
	for _, email := range emails {
		if _, err := authDB.Create(email, count); err != nil {
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
	if len(args) > 0 {
		if authCreateCfg.batchCfg.EmailsPath != "" {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args
	} else {
		list, err := ioutil.ReadFile(authCreateCfg.batchCfg.EmailsPath)
		if err != nil {
			return errs.Wrap(err)
		}
		emails = strings.Split(string(list), authCreateCfg.Delimiter)
	}

	var emailErrs, printErrs utils.ErrorGroup
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "Email\tClaimed\tAvail.\t"); err != nil {
		return err
	}

	for i, email := range emails {
		auths, err := authDB.Get(email)
		if err != nil {
			emailErrs.Add(err)
			continue
		}
		if len(auths) < 1 {
			if i == len(emails)-1 {
				if _, err := fmt.Fprintln(w, "No authorizations for requested email(s)"); err != nil {
					return errs.Wrap(err)
				}
			}
			continue
		}

		claimed, open := auths.Group()
		if _, err := fmt.Fprintf(w,
			"%s\t%d\t%d\t\n",
			email,
			len(claimed),
			len(open),
		); err != nil {
			printErrs.Add(err)
		}
	}

	if err := w.Flush(); err != nil {
		return errs.Wrap(err)
	}
	return utils.CombineErrors(emailErrs.Finish(), printErrs.Finish())
}

func main() {
	process.Exec(rootCmd)
}
