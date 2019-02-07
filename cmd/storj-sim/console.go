// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console/consoleauth"
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

func generateActivationKey(userID uuid.UUID, email string, createdAt time.Time) (string, error) {
	claims := consoleauth.Claims{
		ID:         userID,
		Email:      email,
		Expiration: createdAt.Add(24 * time.Hour),
	}

	signer := &consoleauth.Hmac{Secret: []byte("my-suppa-secret-key")}

	json, err := claims.JSON()
	if err != nil {
		return "", err
	}

	token := consoleauth.Token{Payload: json}
	encoded := base64.URLEncoding.EncodeToString(token.Payload)

	signature, err := signer.Sign([]byte(encoded))
	if err != nil {
		return "", err
	}

	token.Signature = signature

	return token.String(), nil
}

func addExampleProjectWithKey(key *string, address string) error {
	client := http.Client{}

	// create user
	var user struct {
		CreateUser struct {
			Email     string
			CreatedAt time.Time
			ID        string
		}
	}
	{
		createUserQuery := fmt.Sprintf(
			"mutation {createUser(input:{email:\"%s\",password:\"%s\",firstName:\"%s\",lastName:\"\"}){id,email,createdAt}}",
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

		err = graphqlDo(&client, request, &user)
		if err != nil {
			return err
		}
	}

	var token struct {
		ActivateAccount string
	}
	{
		userID, err := uuid.Parse(user.CreateUser.ID)
		if err != nil {
			return err
		}

		activationToken, err := generateActivationKey(*userID, user.CreateUser.Email, user.CreateUser.CreatedAt)
		if err != nil {
			return err
		}

		activateAccountQuery := fmt.Sprintf(
			"mutation {activateAccount(input:\"%s\")}",
			activationToken)

		request, err := http.NewRequest(
			http.MethodPost,
			address,
			bytes.NewReader([]byte(activateAccountQuery)))

		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")

		err = graphqlDo(&client, request, &token)
		if err != nil {
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
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.ActivateAccount))

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
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.ActivateAccount))

		if err := graphqlDo(&client, request, &createAPIKey); err != nil {
			return err
		}
	}

	// return key to the caller
	*key = createAPIKey.CreateAPIKey.Key
	return nil
}
