// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
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
		return []Bucket{}, UnexpectedStatusCode.New(strconv.Itoa(resp.StatusCode))
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Bucket{}, err
	}
	var buckets []Bucket
	err = json.Unmarshal(b, &buckets)
	if err != nil {
		return buckets, err
	}
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
