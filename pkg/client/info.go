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

package storj

import (
	"encoding/json"
	"errors"
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

// UnmarshalJSON overrides the unmarshalling for Info to correctly extract the data from the Swagger JSON response
func (info *Info) UnmarshalJSON(b []byte) error {
	var f interface{}
	json.Unmarshal(b, &f)
	m := f.(map[string]interface{})
	v, ok := m["info"]
	if !ok {
		return errors.New("Missing info element in JSON response")
	}
	i := v.(map[string]interface{})
	v, ok = i["title"]
	if !ok {
		return errors.New("Missing title element in JSON response")
	}
	info.Title = v.(string)
	v, ok = i["description"]
	if !ok {
		return errors.New("Missing description element in JSON response")
	}
	info.Description = v.(string)
	v, ok = i["version"]
	if !ok {
		return errors.New("Missing version element in JSON response")
	}
	info.Version = v.(string)
	v, ok = m["host"]
	if !ok {
		return errors.New("Missing host element in JSON response")
	}
	info.Host = v.(string)
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
