// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"strconv"
	"strings"

	"github.com/zeebo/errs"
)

// Labeled is one entry of a labeled list, such as a metabase connection string
// or the PD endpoints of the cluster behind it.
type Labeled struct {
	Label string
	Value string
}

// SplitLabeled splits a sep-separated list into labeled entries, tolerating the
// spaces a list written by hand in a config file tends to pick up.
//
// An entry may be prefixed with "label=" to name it. Entries without one are
// labeled with their position, which is how they were addressed back when
// everything was configured by index, so an unlabeled list keeps working
// unchanged and mixing the two forms stays consistent. An "=" that comes after
// a ":" belongs to the value -- a query parameter of a connection string, say
// -- rather than introducing a label.
func SplitLabeled(list, sep string) ([]Labeled, error) {
	var entries []Labeled
	seen := map[string]bool{}
	for _, entry := range strings.Split(list, sep) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		labeled := Labeled{Label: strconv.Itoa(len(entries)), Value: entry}
		if eq := strings.Index(entry, "="); eq >= 0 {
			if colon := strings.Index(entry, ":"); colon < 0 || eq < colon {
				labeled = Labeled{
					Label: strings.TrimSpace(entry[:eq]),
					Value: strings.TrimSpace(entry[eq+1:]),
				}
			}
		}

		if labeled.Label == "" || labeled.Value == "" {
			return nil, errs.New("entry %q has an empty label or value", entry)
		}
		if !ValidLabel(labeled.Label) {
			return nil, errs.New("label %q must consist of letters, digits, '-' or '_'", labeled.Label)
		}
		if seen[labeled.Label] {
			return nil, errs.New("duplicate label %q", labeled.Label)
		}
		seen[labeled.Label] = true

		entries = append(entries, labeled)
	}
	return entries, nil
}

// ValidLabel reports whether a label is safe to name a backend with elsewhere.
//
// Other settings refer to a backend by label inside lists of their own -- the
// comma-separated pairs of project-to-adapter, for one -- so a label carrying a
// separator of its own would name a backend that nothing else can spell. Keep
// labels to plain identifiers rather than teaching every list to quote them.
func ValidLabel(label string) bool {
	for _, r := range label {
		switch {
		case 'a' <= r && r <= 'z', 'A' <= r && r <= 'Z', '0' <= r && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
