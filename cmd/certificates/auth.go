// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/utils"
)

var (
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

	authInfoCmd = &cobra.Command{
		Use:   "info [<email>, ...]",
		Short: "Get authorization(s) info from CSR authorization DB",
		RunE:  cmdInfoAuth,
	}

	authExportCmd = &cobra.Command{
		Use:   "export [<email>, ...]",
		Short: "Export authorization(s) from CSR authorization DB to a CSV file (or stdout)",
		RunE:  cmdExportAuth,
	}
)

func parseEmailsList(fileName, delimiter string) (emails []string, err error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		emails = append(emails, line)
	}
	return emails, file.Close()
}

func cmdCreateAuth(cmd *cobra.Command, args []string) error {
	count, err := strconv.Atoi(args[0])
	if err != nil {
		return errs.New("Count couldn't be parsed: %s", args[0])
	}
	authDB, err := config.Signer.NewAuthDB()
	if err != nil {
		return err
	}

	var emails []string
	if len(args) > 1 {
		if config.EmailsPath != "" {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args[1:]
	} else {
		emails, err = parseEmailsList(config.EmailsPath, config.Delimiter)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	var incErrs utils.ErrorGroup
	for _, email := range emails {
		if _, err := authDB.Create(email, count); err != nil {
			incErrs.Add(err)
		}
	}
	return incErrs.Finish()
}

func cmdInfoAuth(cmd *cobra.Command, args []string) error {
	authDB, err := config.Signer.NewAuthDB()
	if err != nil {
		return err
	}

	var emails []string
	if len(args) > 0 {
		if config.EmailsPath != "" && !config.All {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args
	} else if len(args) == 0 || config.All {
		emails, err = authDB.UserIDs()
		if err != nil {
			return err
		}
	} else if _, err := os.Stat(config.EmailsPath); err != nil {
		return errs.New("Emails path error: %s", err)
	} else {
		emails, err = parseEmailsList(config.EmailsPath, config.Delimiter)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	var emailErrs, printErrs utils.ErrorGroup
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "Email\tClaimed\tAvail.\t"); err != nil {
		return err
	}

	for _, email := range emails {
		if err := writeAuthInfo(authDB, email, w); err != nil {
			emailErrs.Add(err)
			continue
		}
	}

	if err := w.Flush(); err != nil {
		return errs.Wrap(err)
	}
	return utils.CombineErrors(emailErrs.Finish(), printErrs.Finish())
}

func writeAuthInfo(authDB *certificates.AuthorizationDB, email string, w io.Writer) error {
	auths, err := authDB.Get(email)
	if err != nil {
		return err
	}
	if len(auths) < 1 {
		return nil
	}

	claimed, open := auths.Group()
	if _, err := fmt.Fprintf(w,
		"%s\t%d\t%d\t\n",
		email,
		len(claimed),
		len(open),
	); err != nil {
		return err
	}

	if config.ShowTokens {
		if err := writeTokenInfo(claimed, open, w); err != nil {
			return err
		}
	}
	return nil
}

func writeTokenInfo(claimed, open certificates.Authorizations, w io.Writer) error {
	groups := map[string]certificates.Authorizations{
		"Claimed": claimed,
		"Open":    open,
	}
	for label, group := range groups {
		if _, err := fmt.Fprintf(w, "\t%s:\n", label); err != nil {
			return err
		}
		if len(group) > 0 {
			for _, auth := range group {
				if _, err := fmt.Fprintf(w, "\t\t%s\n", auth.Token.String()); err != nil {
					return err
				}
			}
		} else {
			if _, err := fmt.Fprintln(w, "\t\tnone"); err != nil {
				return err
			}
		}
	}
	return nil
}

func cmdExportAuth(cmd *cobra.Command, args []string) error {
	authDB, err := config.Signer.NewAuthDB()
	if err != nil {
		return err
	}

	var emails []string
	if len(args) > 0 && !config.All {
		if config.EmailsPath != "" {
			return errs.New("Either use `--emails-path` or positional args, not both.")
		}
		emails = args
	} else if len(args) == 0 || config.All {
		emails, err = authDB.UserIDs()
		if err != nil {
			return err
		}
	} else {
		emails, err = parseEmailsList(config.EmailsPath, config.Delimiter)
		if err != nil {
			return errs.Wrap(err)
		}
	}

	var (
		emailErrs, csvErrs utils.ErrorGroup
		output             io.Writer
	)
	switch config.Out {
	case "-":
		output = os.Stdout
	default:
		if err := os.MkdirAll(filepath.Dir(config.Out), 0600); err != nil {
			return errs.Wrap(err)
		}
		output, err = os.OpenFile(config.Out, os.O_CREATE, 0600)
		if err != nil {
			return errs.Wrap(err)
		}
	}
	csvWriter := csv.NewWriter(output)

	for _, email := range emails {
		if err := writeAuthExport(authDB, email, csvWriter); err != nil {
			emailErrs.Add(err)
		}
	}

	csvWriter.Flush()
	return utils.CombineErrors(emailErrs.Finish(), csvErrs.Finish())
}

func writeAuthExport(authDB *certificates.AuthorizationDB, email string, w *csv.Writer) error {
	auths, err := authDB.Get(email)
	if err != nil {
		return err
	}
	if len(auths) < 1 {
		return nil
	}

	var authErrs utils.ErrorGroup
	for _, auth := range auths {
		if err := w.Write([]string{email, auth.Token.String()}); err != nil {
			authErrs.Add(err)
		}
	}
	return authErrs.Finish()
}
