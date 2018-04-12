// Copyright (C) 2018 Storj Labs, Inc.
//
// This file is part of the Storj client library.
//
// The Storj client library is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// The Storj client library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with The Storj client library.  If not, see
// <http://www.gnu.org/licenses/>.

package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// Info struct of the GetInfo() response
type Info struct {
	Title       string
	Description string
	Version     string
	Host        string
}

type swagger struct {
	Info swaggerInfo
	Host string
}

type swaggerInfo struct {
	Title       string
	Description string
	Version     string
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
		return info, fmt.Errorf("Unexpected response code: %d", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return info, err
	}
	err = json.Unmarshal(b, &info)
	return info, err
}
