// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import "strings"

// HostSet represents a set of hosts. It is used for trust filtering.
type HostSet struct {
	exact    map[string]struct{}
	suffixes map[string]struct{}
}

// NewHostSet returns a new host set.
func NewHostSet() *HostSet {
	return &HostSet{
		exact:    make(map[string]struct{}),
		suffixes: make(map[string]struct{}),
	}
}

// Add adds a host to the host set.
func (set *HostSet) Add(host string) bool {
	host = normalizeHost(host)
	if host == "" {
		return false
	}

	set.exact[host] = struct{}{}
	isDomain := strings.ContainsRune(host, '.')
	if isDomain {
		set.suffixes["."+host] = struct{}{}
	}
	return true
}

// Includes returns true if the host belongs to the host set or false
// otherwise.  A host that is a subdomain of an host belonging to the host set
// also belongs to the set (i.e. sub.domain.test will be considered "included"
// if domain.test was added to the set)
func (set HostSet) Includes(host string) bool {
	host = normalizeHost(host)
	if host == "" {
		return false
	}

	if _, ok := set.exact[host]; ok {
		return true
	}

	for suffix := range set.suffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

func normalizeHost(host string) string {
	return strings.ToLower(strings.Trim(host, "."))
}
