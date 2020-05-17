package main

import (
	"encoding/json"
	"fmt"
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
	var resp map[string]interface{}
	var err error

	_ = r.ParseForm()
	fmt.Println(r.Form)
	request := r.Form["request"][0]

	switch request {
	case "userinfo":
		resp, err = makeRequest("GET", fmt.Sprintf("api/user/%s", r.Form["email"][0]), "")
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
		}
	case "usercreate":
		resp, err = makeRequest("POST", "api/user/", fmt.Sprintf("?email=%s&password=%s", r.Form["email"][0], r.Form["password"][0]))
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
		}
	case "projectinfo":
		resp, err = makeRequest("GET", fmt.Sprintf("api/project/%s/limit", r.Form["projectid"][0]), "")
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
		}
	case "projectcreate":
		resp, err = makeRequest("POST", "api/project/", fmt.Sprintf("?projectName=%s&ownerID=%s", r.Form["projectname"][0], r.Form["ownerid"][0]))
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
		}
	}
	resp["method"] = request
	_ = template.Public.Execute(w, resp)
}

func makeRequest(method, path, query string) (res map[string]interface{}, err error) {
	queryEndpoint, err := url.Parse(runCfg.EndpointURL)
	if err != nil {
		return nil, err
	}
	queryEndpoint.Path = path
	if query != "" {
		queryEndpoint.RawQuery = url.PathEscape(query)
	}
	fmt.Println(queryEndpoint.String())

	req, err := http.NewRequest(method, queryEndpoint.String(), nil)
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
