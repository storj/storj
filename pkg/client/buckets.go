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
	"log"
	"net/http"
)

// Bucket metdata
type Bucket struct {
	ID        string
	Name      string
	Created   string
	Decrypted bool
}

// GetBuckets returns the list of buckets
func GetBuckets(env Env) ([]Bucket, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", env.URL+"/buckets", nil)
	if err != nil {
		return []Bucket{}, err
	}
	req.SetBasicAuth(env.User, env.Password)
	resp, err := client.Do(req)
	if err != nil {
		return []Bucket{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return []Bucket{}, fmt.Errorf("Unexpected response code: %d", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Bucket{}, err
	}
	var buckets []Bucket
	err = json.Unmarshal(b, &buckets)
	for i, b := range buckets {
		decryptedName, err := decryptBucketName(b.Name, env.Mnemonic)
		if err != nil {
			log.Printf("Could not decrypt bucket name %s: %s\n", b.Name, err)
			continue
		}
		buckets[i].Name = decryptedName
		buckets[i].Decrypted = true
	}
	return buckets, err
}
