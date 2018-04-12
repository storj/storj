// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
)

// Info struct of the GetInfo() response
type Info struct {
	Title       string
	Description string
	Version     string
	Host        string
}

type swagger struct {
	Info struct {
		Title       string
		Description string
		Version     string
	}
	Host string
}

// UnmarshalJSON overrides the unmarshalling for Info to correctly extract the data from the Swagger JSON response
func (info *Info) UnmarshalJSON(b []byte) error {
	var s swagger
	json.Unmarshal(b, &s)
	info.Title = s.Info.Title
	info.Description = s.Info.Description
	info.Version = s.Info.Version
	info.Host = s.Host
	return nil
}

// GetInfo returns info about the Storj Bridge server
func GetInfo(env Env) (Info, error) {
	info := Info{}
	resp, err := http.Get(env.URL)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, UnexpectedStatusCode.New(strconv.Itoa(resp.StatusCode))
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}
	err = json.Unmarshal(b, &info)
	return info, err
}
