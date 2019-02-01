package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

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

func cmdClaims(cmd *cobra.Command, args []string) error {
	authDB, err := claimsCfg.NewAuthDB()
	if err != nil {
		return err
	}

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
	return nil
}

type printableAuth struct {
	UserID string
	Token  string
	Claim  *pClaim
}
type pClaim struct {
	Addr      string
	Timestamp string
	NodeID    string
}

func toPrintableAuth(a *certificates.Authorization) *printableAuth {
	pAuth := new(printableAuth)

	pAuth.UserID = a.Token.UserID
	pAuth.Token = a.Token.String()

	if a.Claim != nil {
		pAuth.Claim = &pClaim{
			Timestamp: time.Unix(a.Claim.Timestamp, 0).String(),
			Addr:      a.Claim.Addr,
			NodeID:    a.Claim.Identity.ID.String(),
		}
	}
	return pAuth
}
