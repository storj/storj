// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/spf13/cobra"

	"storj.io/storj/cmd/storj-admin/template"
)

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	handler := mux.NewRouter()
	handler.HandleFunc("/", serveRoot)

	server := http.Server{Addr: runCfg.Address, Handler: handler}
	err = server.ListenAndServe()
	return err
}

func serveRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		err := template.Public.Execute(w, nil)
		if err != nil {
			log.Println(err)
		}
		return
	}
	handleRequest(w, r)
}

func handleRequest(w http.ResponseWriter, r *http.Request) {
	var err error

	resp := map[string]interface{}{}

	_ = r.ParseForm()
	request := r.Form["request"][0]

	switch request {
	case "userinfo":
		resp, err = makeRequest("GET", fmt.Sprintf("api/user/%s", r.Form["email"][0]), "", nil)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "usercreate":
		input := struct {
			Email    string `json:"email"`
			Name     string `json:"name"`
			Password string `json:"password"`
		}{
			Email: r.Form["email"][0],
			Name:  r.Form["name"][0],
		}

		byteJson, err := json.Marshal(input)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err = makeRequest("POST", "api/user", "", bytes.NewReader(byteJson))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "projectinfo":
		resp, err = makeRequest("GET", fmt.Sprintf("api/project/%s/limit", r.Form["projectid"][0]), "", nil)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "projectcreate":
		input := struct {
			OwnerID     string `json:"ownerId"`
			ProjectName string `json:"projectName"`
		}{
			OwnerID:     r.Form["ownerId"][0],
			ProjectName: r.Form["projectName"][0],
		}

		byteJson, err := json.Marshal(input)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err = makeRequest("POST", "api/project", "", bytes.NewReader(byteJson))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
	resp["method"] = request
	_ = template.Public.Execute(w, resp)
}

func makeRequest(method, path, query string, body io.Reader) (res map[string]interface{}, err error) {
	queryEndpoint, err := url.Parse(runCfg.EndpointURL)
	if err != nil {
		return nil, err
	}
	queryEndpoint.Path = path
	if query != "" {
		queryEndpoint.RawQuery = url.PathEscape(query)
	}

	req, err := http.NewRequest(method, queryEndpoint.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", runCfg.AuthKey)

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&res)
	if err != nil {
		return nil, err
	}

	return res, nil
}
