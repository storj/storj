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

	// TODO: change it in future, when satellite/console secret will be changed
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

func addExampleProjectWithKey(key *string, endpoints map[string]string) error {
	client := http.Client{}

	var createTokenResponse struct {
		Secret string
		Error  string
	}
	{
		request, err := http.NewRequest(
			http.MethodGet,
			endpoints["regtoken"],
			nil)
		if err != nil {
			return err
		}
		request.Header.Set("Authorization", "secure_token")

		resp, err := client.Do(request)
		if err != nil {
			return err
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		if err = json.NewDecoder(bytes.NewReader(b)).Decode(&createTokenResponse); err != nil {
			return err
		}

		if createTokenResponse.Error != "" {
			return errs.New(createTokenResponse.Error)
		}

		err = resp.Body.Close()
		if err != nil {
			return err
		}
	}

	// create user
	var user struct {
		CreateUser struct {
			Email     string
			CreatedAt time.Time
			ID        string
		}
	}
	{
		var registerData struct {
			FullName    string `json:"fullName"`
			ShortName   string `json:"shortName"`
			Email       string `json:"email"`
			Password    string `json:"password"`
			SecretInput string `json:"secret"`
		}

		registerData.FullName = "Alice"
		registerData.Email = "alice@mail.test"
		registerData.Password = "123a123"
		registerData.ShortName = "al"
		registerData.SecretInput = createTokenResponse.Secret

		res, _ := json.Marshal(registerData)

		request, err := http.NewRequest(
			http.MethodPost,
			endpoints["register"],
			bytes.NewReader(res))
		if err != nil {
			return err
		}
		request.Header.Add("Content-Type", "application/json")

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		defer func() { err = errs.Combine(err, response.Body.Close()) }()

		if response.StatusCode != http.StatusOK {
			return err
		}

		err = json.NewDecoder(response.Body).Decode(&user.CreateUser.ID)
		if err != nil {
			return err
		}

		user.CreateUser.Email = registerData.Email
		user.CreateUser.CreatedAt = time.Now()
	}

	var token string
	{
		userID, err := uuid.Parse(user.CreateUser.ID)
		if err != nil {
			return err
		}

		activationToken, err := generateActivationKey(*userID, user.CreateUser.Email, user.CreateUser.CreatedAt)
		if err != nil {
			return err
		}

		request, err := http.NewRequest(
			http.MethodGet,
			endpoints["activation"]+activationToken,
			nil)
		if err != nil {
			return err
		}

		resp, err := client.Do(request)
		if err != nil {
			return err
		}

		defer func() { err = errs.Combine(err, resp.Body.Close()) }()

		var authToken struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		authToken.Email = "alice@mail.test"
		authToken.Password = "123a123"

		res, _ := json.Marshal(authToken)

		request, err = http.NewRequest(
			http.MethodPost,
			endpoints["token"],
			bytes.NewReader(res))
		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/json")

		response, err := client.Do(request)
		if err != nil {
			return err
		}

		defer func() { err = errs.Combine(err, response.Body.Close()) }()

		if response.StatusCode != http.StatusOK {
			return err
		}

		err = json.NewDecoder(response.Body).Decode(&token)
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
			endpoints["graphql"],
			bytes.NewReader([]byte(createProjectQuery)))
		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

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
			endpoints["graphql"],
			bytes.NewReader([]byte(createAPIKeyQuery)))
		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

		if err := graphqlDo(&client, request, &createAPIKey); err != nil {
			return err
		}
	}

	// return key to the caller
	*key = createAPIKey.CreateAPIKey.Key
	return nil
}
