// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import "storj.io/storj/shared/mud"

// Subcommand is a mud annotation for components, which can work as subcommands.
type Subcommand struct {
	Name        string
	Description string
}

// RegisterSubcommand registers a subcommand with the given name and description.
func RegisterSubcommand[T any](ball *mud.Ball, name, description string) {
	mud.Tag[T, Subcommand](ball, Subcommand{
		Name:        name,
		Description: description,
	})
}
