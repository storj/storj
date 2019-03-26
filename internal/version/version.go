package version

import "encoding/json"

type Info struct {
	Timestamp  string `json:"timestamp"`
	CommitHash string `json:"hash"`
	Version    string `json:"version"`
	Release    bool   `json:"release"`
}

// New creates Version_Info from a json byte array
func New(data []byte) (v Info, err error) {
	err = json.Unmarshal(data, v)
	return v, err
}

// Marshal converts the existing Version Info to any json byte array
func (v Info) Marshal() (data []byte, err error) {
	data, err = json.Marshal(v)
	return
}
