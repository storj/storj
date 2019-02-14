// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/cfgstruct"
)

var (
	claimsCmd = &cobra.Command{
		Use:   "claims",
		Short: "CSR authorization claim management",
	}

	claimsExportCmd = &cobra.Command{
		Use:   "export",
		Short: "Export all claim data as JSON",
		RunE:  cmdExportClaims,
	}

	claimDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete a claim on an authorization",
		Args: cobra.ExactArgs(1),
		RunE:  cmdDeleteClaim,
	}

	claimsExportCfg struct {
		certificates.CertServerConfig
		Raw bool `default:"false" help:"if true, the raw data structures will be printed"`
	}

	claimDeleteCfg certificates.CertServerConfig
)

func init() {
	rootCmd.AddCommand(claimsCmd)
	claimsCmd.AddCommand(claimsExportCmd)
	claimsCmd.AddCommand(claimDeleteCmd)

	cfgstruct.Bind(claimsExportCmd.Flags(), &claimsExportCfg, cfgstruct.ConfDir(defaultConfDir))
	cfgstruct.Bind(claimDeleteCmd.Flags(), &claimDeleteCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdExportClaims(cmd *cobra.Command, args []string) (err error) {
	authDB, err := claimsExportCfg.NewAuthDB()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, authDB.Close())
	}()

	auths, err := authDB.List()
	if err != nil {
		return err
	}

	var toPrint []interface{}
	for _, auth := range auths {
		if auth.Claim == nil {
			continue
		}

		if claimsExportCfg.Raw {
			toPrint = append(toPrint, auth)
		} else {
			toPrint = append(toPrint, toPrintableAuth(auth))
		}
	}

	if len(toPrint) == 0 {
		fmt.Printf("no claims in database: %s\n", claimsExportCfg.AuthorizationDBURL)
		return nil
	}

	jsonBytes, err := json.MarshalIndent(toPrint, "", "\t")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return err
}

func cmdDeleteClaim(cmd *cobra.Command, args [] string) (err error) {
	authDB, err := claimsExportCfg.NewAuthDB()
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, authDB.Close())
	}()

	if err := authDB.Unclaim(args[0]); err != nil {
		return err
	}
	return nil
}

type printableAuth struct {
	UserID string
	Token  string
	Claim  *printableClaim
}
type printableClaim struct {
	Addr   string
	Time   string
	NodeID string
}

func toPrintableAuth(auth *certificates.Authorization) *printableAuth {
	pAuth := new(printableAuth)

	pAuth.UserID = auth.Token.UserID
	pAuth.Token = auth.Token.String()

	if auth.Claim != nil {
		pAuth.Claim = &printableClaim{
			Time:   time.Unix(auth.Claim.Timestamp, 0).String(),
			Addr:   auth.Claim.Addr,
			NodeID: auth.Claim.Identity.ID.String(),
		}
	}
	return pAuth
}
