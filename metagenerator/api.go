package metagenerator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"

	"storj.io/storj/metasearch"
)

type Request struct {
	Path  string         `json:"path"`
	Match map[string]any `json:""`
}

type Response struct {
	Results []Record `json:"results"`
}

func putMeta(record *Record, apiKey, projectId, url string) error {
	req, err := json.Marshal(record)
	if err != nil {
		return err
	}

	r, err := http.NewRequest("PUT", url, bytes.NewBuffer(req))
	if err != nil {
		return err
	}
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	r.Header.Add("X-Project-ID", projectId)

	/* TODO: add back with log levels
	re, err := httputil.DumpRequest(r, true)
	if err != nil {
		return err
	}
	fmt.Println(string(re))
	*/

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return err
	}

	return nil
}

func SearchMeta(query metasearch.SearchRequest, apiKey, projectId, url string) (bodyBytes []byte, err error) {
	req, err := json.Marshal(query)
	if err != nil {
		return
	}
	r, err := http.NewRequest("POST", url, bytes.NewBuffer(req))
	if err != nil {
		return
	}
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	r.Header.Add("X-Project-ID", projectId)

	reqest, err := httputil.DumpRequest(r, true)
	if err != nil {
		return
	}
	fmt.Println(string(reqest))

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return
	}
	defer res.Body.Close()

	bodyBytes, err = io.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}

	return
}
