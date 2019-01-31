// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
)

type graphqlResponse struct {
	Data   []byte
	Errors []interface{}
}

func graphqlDo(client *http.Client, request *http.Request) (graphqlResponse, error) {
	resp, err := client.Do(request)
	if err != nil {
		return graphqlResponse{}, err
	}
	defer func() {
		err = resp.Body.Close()
	}()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return graphqlResponse{}, err
	}

	fmt.Println(string(b))

	var response struct {
		Data   json.RawMessage
		Errors []interface{}
	}

	err = json.NewDecoder(bytes.NewReader(b)).Decode(&response)
	if err != nil {
		return graphqlResponse{}, err
	}

	return graphqlResponse{
		Data:   []byte(response.Data),
		Errors: response.Errors,
	}, nil
}

func addExampleProjectWithKey(key *string, address string) error {
	client := http.Client{}

	var (
		createUserFormat    = "mutation {createUser(input:{email:\"%s\",password:\"%s\",firstName:\"%s\",lastName:\"%s\"})}"
		createProjectFormat = "mutation {createProject(input:{name:\"%s\",description:\"%s\"}){id}}"
		createAPIKeyFormat  = "mutation {createAPIKey(projectID:\"%s\",name:\"%s\"){key}}"
		tokenFormat         = "query {token(email:\"%s\",password:\"%s\"){token}}"
	)

	alice := console.CreateUser{
		UserInfo: console.UserInfo{
			FirstName: "Alice",
			Email:     "example@mail.com",
		},
		Password: "123a123",
	}

	testProject := console.ProjectInfo{
		Name: "TestProject",
	}

	testKey := console.APIKeyInfo{
		Name: "testKey",
	}

	createUserQuery := fmt.Sprintf(
		createUserFormat,
		alice.Email,
		alice.Password,
		alice.FirstName,
		alice.LastName)

	createProjectQuery := fmt.Sprintf(
		createProjectFormat,
		testProject.Name,
		testProject.Description)

	tokenQuery := fmt.Sprintf(
		tokenFormat,
		alice.Email,
		alice.Password)

	// create user
	{
		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(createUserQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")

		resp, err := graphqlDo(&client, request)
		if err != nil {
			return err
		}

		if resp.Errors != nil {
			return errs.New("inner graphql error")
		}
	}

	// get token
	var token struct {
		Token struct {
			Token string
		}
	}
	{
		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(tokenQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")

		resp, err := graphqlDo(&client, request)
		if err != nil {
			return err
		}

		if resp.Errors != nil {
			return errs.New("inner graphql error")
		}

		if err = json.NewDecoder(bytes.NewReader(resp.Data)).Decode(&token); err != nil {
			return err
		}
	}

	// create project
	var createProject struct {
		CreateProject struct {
			ID string
		}
	}
	{
		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(createProjectQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.Token.Token))

		resp, err := graphqlDo(&client, request)
		if err != nil {
			return err
		}

		if resp.Errors != nil {
			return errs.New("inner graphql error")
		}

		if err = json.NewDecoder(bytes.NewReader(resp.Data)).Decode(&createProject); err != nil {
			return err
		}
	}

	// create api key
	createAPIKeyQuery := fmt.Sprintf(
		createAPIKeyFormat,
		createProject.CreateProject.ID,
		testKey.Name)

	var createAPIKey struct {
		CreateAPIKey struct {
			Key string
		}
	}
	{
		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(createAPIKeyQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.Token.Token))

		resp, err := graphqlDo(&client, request)
		if err != nil {
			return err
		}

		if resp.Errors != nil {
			return errs.New("inner graphql error")
		}

		if err = json.NewDecoder(bytes.NewReader(resp.Data)).Decode(&createAPIKey); err != nil {
			return err
		}
	}

	// return key to the caller
	*key = createAPIKey.CreateAPIKey.Key
	return nil
}
