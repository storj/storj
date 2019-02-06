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
		Short: "Print claim information",
		RunE:  cmdClaims,
	}

	claimsCfg struct {
		certificates.CertServerConfig
		Raw bool `default:"false" help:"if true, the raw data structures will be printed"`
	}
)

func init() {
	rootCmd.AddCommand(claimsCmd)

	cfgstruct.Bind(claimsCmd.Flags(), &claimsCfg, cfgstruct.ConfDir(defaultConfDir))
}

func cmdClaims(cmd *cobra.Command, args []string) (err error) {
	authDB, err := claimsCfg.NewAuthDB()
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

		if claimsCfg.Raw {
			toPrint = append(toPrint, auth)
		} else {
			toPrint = append(toPrint, toPrintableAuth(auth))
		}
	}

	if len(toPrint) == 0 {
		fmt.Printf("no claims in database: %s\n", claimsCfg.AuthorizationDBURL)
		return nil
	}

	jsonBytes, err := json.MarshalIndent(toPrint, "", "\t")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return err
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
