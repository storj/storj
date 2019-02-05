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
)

func graphqlDo(client *http.Client, request *http.Request, jsonResponse interface{}) error {
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer func() {
		err = resp.Body.Close()
	}()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var response struct {
		Data   json.RawMessage
		Errors []interface{}
	}

	if err = json.NewDecoder(bytes.NewReader(b)).Decode(&response); err != nil {
		return err
	}

	if response.Errors != nil {
		return errs.New("inner graphql error")
	}

	if jsonResponse == nil {
		return nil
	}

	return json.NewDecoder(bytes.NewReader(response.Data)).Decode(jsonResponse)
}

func addExampleProjectWithKey(key *string, address string) error {
	client := http.Client{}

	// create user
	{
		createUserQuery := fmt.Sprintf(
			"mutation {createUser(input:{email:\"%s\",password:\"%s\",firstName:\"%s\",lastName:\"\"})}",
			"example@mail.com",
			"123a123",
			"Alice")

		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(createUserQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")

		if err := graphqlDo(&client, request, nil); err != nil {
			return err
		}
	}

	// get token
	var token struct {
		Token struct {
			Token string
		}
	}
	{
		tokenQuery := fmt.Sprintf(
			"query {token(email:\"%s\",password:\"%s\"){token}}",
			"example@mail.com",
			"123a123")

		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(tokenQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")

		if err := graphqlDo(&client, request, &token); err != nil {
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
		createProjectQuery := fmt.Sprintf(
			"mutation {createProject(input:{name:\"%s\",description:\"\"}){id}}",
			"TestProject")

		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(createProjectQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.Token.Token))

		if err := graphqlDo(&client, request, &createProject); err != nil {
			return err
		}
	}

	// create api key
	var createAPIKey struct {
		CreateAPIKey struct {
			Key string
		}
	}
	{
		createAPIKeyQuery := fmt.Sprintf(
			"mutation {createAPIKey(projectID:\"%s\",name:\"%s\"){key}}",
			createProject.CreateProject.ID,
			"testKey")

		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(createAPIKeyQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.Token.Token))

		if err := graphqlDo(&client, request, &createAPIKey); err != nil {
			return err
		}
	}

	// return key to the caller
	*key = createAPIKey.CreateAPIKey.Key
	return nil
}
