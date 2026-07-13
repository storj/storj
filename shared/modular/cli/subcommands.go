// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import "storj.io/storj/shared/mud"

// Subcommand is a mud annotation for components, which can work as subcommands.
type Subcommand struct {
	// Group is the optional parent command. When set, the subcommand is nested
	// under it (e.g. `satellite compensation generate-invoices`). Empty means
	// the subcommand is registered at the top level.
	Group            string
	GroupDescription string
	Name             string
	Description      string
}

// SubcommandGroup identifies a parent command under which related subcommands are
// nested (e.g. `satellite compensation ...`). It is passed to
// RegisterGroupSubcommand so the group name and description don't have to be
// repeated for every subcommand.
type SubcommandGroup struct {
	Name        string
	Description string
}

// RegisterSubcommand registers a top-level subcommand with the given name and description.
func RegisterSubcommand[T any](ball *mud.Ball, name, description string) {
	mud.Tag[T, Subcommand](ball, Subcommand{
		Name:        name,
		Description: description,
	})
}

// RegisterGroupSubcommand registers a subcommand nested under the given group, so
// it is invoked as `<group> <name>`. All subcommands sharing the same group are
// collected under a single parent command; the group description is used for the
// parent command help (the first non-empty value registered for the group wins).
func RegisterGroupSubcommand[T any](ball *mud.Ball, group SubcommandGroup, name, description string) {
	mud.Tag[T, Subcommand](ball, Subcommand{
		Group:            group.Name,
		GroupDescription: group.Description,
		Name:             name,
		Description:      description,
	})
}
