package version

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

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

func QueryVersionFromControllServer() (ver Info, err error) {
	resp, err := http.Get("https://version.storj.io/")
	if err != nil {
		return Info{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Info{}, err
	}

	ver, err = New(body)
	return
}
