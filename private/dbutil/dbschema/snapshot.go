// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbschema

import (
	"bufio"
	"sort"
	"strings"
)

// Snapshots defines a collection of snapshot.
type Snapshots struct {
	List []*Snapshot
}

// Snapshot defines a particular snapshot of schema and data.
type Snapshot struct {
	Version int
	Sections
	*Schema
	*Data
}

// Add adds a new snapshot.
func (snapshots *Snapshots) Add(snap *Snapshot) {
	snapshots.List = append(snapshots.List, snap)
}

// FindVersion finds a snapshot with the specified version.
func (snapshots *Snapshots) FindVersion(version int) (*Snapshot, bool) {
	for _, snap := range snapshots.List {
		if snap.Version == version {
			return snap, true
		}
	}
	return nil, false
}

// Sort sorts the snapshots by version.
func (snapshots *Snapshots) Sort() {
	sort.Slice(snapshots.List, func(i, k int) bool {
		return snapshots.List[i].Version < snapshots.List[k].Version
	})
}

// Sections is a type to keep track of the sections inside of a sql script.
type Sections struct {
	Script   string
	Sections map[string]string
}

// These consts are the names of the sections that are typical in our scripts.
const (
	NewData = "NEW DATA"
	OldData = "OLD DATA"
	Main    = "MAIN"
)

// NewSections constructs a Sections from a sql script.
func NewSections(script string) Sections {
	sections := make(map[string]string)

	var buf strings.Builder
	section := "MAIN"
	scanner := bufio.NewScanner(strings.NewReader(script))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > 6 && line[:3] == "-- " && line[len(line)-3:] == " --" {
			sections[section] += buf.String()
			buf.Reset()
			section = line[3 : len(line)-3]
		}
		_, _ = buf.WriteString(line)
		_ = buf.WriteByte('\n')
	}

	if buf.Len() > 0 {
		sections[section] += buf.String()
	}

	return Sections{
		Script:   script,
		Sections: sections,
	}
}

// LookupSection finds the named section in the script or returns an empty string.
func (s Sections) LookupSection(section string) string {
	return s.Sections[section]
}
