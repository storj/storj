// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package dbutil

import (
	"fmt"
	"strings"
)

// SplitConnstr returns the driver and DSN portions of a URL
func SplitConnstr(s string) (string, string, error) {
	// consider https://github.com/xo/dburl if this ends up lacking
	parts := strings.SplitN(s, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Could not parse DB URL %s", s)
	}
	if parts[0] == "postgres" {
		parts[1] = s // postgres wants full URLS for its DSN
	}
	return parts[0], parts[1], nil
}
