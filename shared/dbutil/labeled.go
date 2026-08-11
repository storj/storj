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

// LabeledGroup is one entry of a labeled list whose label may name more than
// one thing at once, for a value that genuinely belongs to all of them -- the
// PD endpoints of a cluster that backs several metabase backends, say.
type LabeledGroup struct {
	Labels []string
	Value  string
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
	groups, err := SplitLabeledGroups(list, sep, "")
	if err != nil {
		return nil, err
	}
	entries := make([]Labeled, 0, len(groups))
	for _, group := range groups {
		entries = append(entries, Labeled{Label: group.Labels[0], Value: group.Value})
	}
	return entries, nil
}

// SplitLabeledGroups splits a sep-separated list the way SplitLabeled does,
// additionally letting one entry carry several labels joined by groupSep:
//
//	west+south=pd-w1:2379,pd-w2:2379;east=pd-e1:2379
//
// which says the value belongs to all of them rather than repeating it once per
// label. Repetition would say something different -- two entries are two things
// that merely look alike -- and callers that act on the thing behind the value
// would then act on it twice. An empty groupSep disables grouping, so every
// entry carries exactly one label.
//
// Labels stay unique across the whole list, grouped or not.
func SplitLabeledGroups(list, sep, groupSep string) ([]LabeledGroup, error) {
	var entries []LabeledGroup
	seen := map[string]bool{}
	for _, entry := range strings.Split(list, sep) {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		group := LabeledGroup{Labels: []string{strconv.Itoa(len(entries))}, Value: entry}
		if eq := strings.Index(entry, "="); eq >= 0 {
			if colon := strings.Index(entry, ":"); colon < 0 || eq < colon {
				group = LabeledGroup{
					Labels: splitLabels(entry[:eq], groupSep),
					Value:  strings.TrimSpace(entry[eq+1:]),
				}
			}
		}

		if group.Value == "" {
			return nil, errs.New("entry %q has an empty label or value", entry)
		}
		for _, label := range group.Labels {
			if label == "" {
				return nil, errs.New("entry %q has an empty label or value", entry)
			}
			if !ValidLabel(label) {
				return nil, errs.New("label %q must consist of letters, digits, '-' or '_'", label)
			}
			if seen[label] {
				return nil, errs.New("duplicate label %q", label)
			}
			seen[label] = true
		}

		entries = append(entries, group)
	}
	return entries, nil
}

// splitLabels splits the label part of an entry into the labels it names.
func splitLabels(labels, groupSep string) []string {
	if groupSep == "" {
		return []string{strings.TrimSpace(labels)}
	}
	parts := strings.Split(labels, groupSep)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
	}
	return parts
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
