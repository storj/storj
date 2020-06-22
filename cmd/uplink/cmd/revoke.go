// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"storj.io/uplink"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "revoke access_here",
		Short: "Revoke an access",
		RunE:  revokeAccess,
		Args:  cobra.ExactArgs(1),
	}, RootCmd)
}

func revokeAccess(cmd *cobra.Command, args []string) error {
	ctx, _ := withTelemetry(cmd)

	if len(args) == 0 {
		return fmt.Errorf("no access specified for revocation")
	}

	accessRaw := args[0]
	access, err := uplink.ParseAccess(accessRaw)
	if err != nil {
		return errors.New("invalid access provided")
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	if err = project.RevokeAccess(ctx, access); err != nil {
		return err
	}
	fmt.Println("=========== SUCCESSFULLY REVOKED =========================================================")
	fmt.Println("NOTE: It may take the satellite several minutes to process the revocation request,")
	fmt.Println("      depending on its caching policies.")

	return nil
}
