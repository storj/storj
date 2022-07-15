// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"encoding/json"
)

// Satellites is a configuration value that contains a list of satellite names and addresses.
// Format should be [{"name": "","address": ""],{"name": "","address": ""},...] in valid JSON format.
//
// Can be used as a flag.
type Satellites []satellite

type satellite struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Type implements pflag.Value.
func (Satellites) Type() string { return "console.Satellites" }

// String is required for pflag.Value.
func (sl *Satellites) String() string {
	satellites, err := json.Marshal(*sl)
	if err != nil {
		return ""
	}

	return string(satellites)
}

// Set does validation on the configured JSON.
func (sl *Satellites) Set(s string) (err error) {
	satellites := make([]satellite, 3)

	err = json.Unmarshal([]byte(s), &satellites)
	if err != nil {
		return err
	}

	*sl = satellites
	return
}
