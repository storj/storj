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
	"strconv"

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
		resp, err = makeRequest(http.MethodGet, fmt.Sprintf("api/user/%s", r.Form["email"][0]), "", nil)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "usercreate":
		input := struct {
			Email    string `json:"email"`
			Name     string `json:"fullName"`
			Password string `json:"password"`
		}{
			Email:    r.Form["email"][0],
			Name:     r.Form["fullName"][0],
			Password: r.Form["password"][0],
		}

		byteJson, err := json.Marshal(input)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err = makeRequest(http.MethodPost, "api/user", "", bytes.NewReader(byteJson))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "projectinfo":
		resp, err = makeRequest(http.MethodGet, fmt.Sprintf("api/project/%s/limit", r.Form["projectid"][0]), "", nil)
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

		resp, err = makeRequest(http.MethodDelete, "api/project", "", bytes.NewReader(byteJson))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "projectdelete":
		resp, err = makeRequest(http.MethodDelete, fmt.Sprintf("api/project/%s", r.Form["projectid"][0]), "", nil)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "couponadd":

		duration, err := strconv.Atoi(r.Form["duration"][0])
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		amount, err := strconv.ParseInt((r.Form["amount"][0]), 10, 64)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		input := struct {
			UserID      string `json:"userid"`
			Duration    int    `json:"duration"`
			Amount      int64  `json:"amount"`
			Description string `json:"description"`
		}{
			UserID:      r.Form["userId"][0],
			Duration:    duration,
			Amount:      amount,
			Description: r.Form["description"][0],
		}

		byteJson, err := json.Marshal(input)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp, err = makeRequest(http.MethodPost, "api/user/coupon", "", bytes.NewReader(byteJson))
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case "couponlist":
		resp, err = makeRequest(http.MethodGet, fmt.Sprintf("api/users/coupon/{userid}", r.Form["userId"][0]), "", nil)
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

	resp, err := http.DefaultClient.Do(req)
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
