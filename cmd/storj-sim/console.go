// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/payments"
)

func newConsoleEndpoints(address string) *consoleEndpoints {
	return &consoleEndpoints{
		client:     http.DefaultClient,
		base:       "http://" + address,
		cookieName: "_tokenKey",
	}
}

type consoleEndpoints struct {
	client     *http.Client
	base       string
	cookieName string
}

func (ce *consoleEndpoints) appendPath(suffix string) string {
	return ce.base + suffix
}

func (ce *consoleEndpoints) RegToken() string {
	return ce.appendPath("/registrationToken/?projectsLimit=1")
}

func (ce *consoleEndpoints) Register() string {
	return ce.appendPath("/api/v0/auth/register")
}

func (ce *consoleEndpoints) SetupAccount() string {
	return ce.appendPath("/api/v0/payments/account")
}

func (ce *consoleEndpoints) CreditCards() string {
	return ce.appendPath("/api/v0/payments/cards")
}

func (ce *consoleEndpoints) Activation(token string) string {
	return ce.appendPath("/activation/?token=" + token)
}

func (ce *consoleEndpoints) Token() string {
	return ce.appendPath("/api/v0/auth/token")
}

func (ce *consoleEndpoints) GraphQL() string {
	return ce.appendPath("/api/v0/graphql")
}

func (ce *consoleEndpoints) graphqlDo(request *http.Request, jsonResponse interface{}) error {
	resp, err := ce.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

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
		return errs.New("inner graphql error: %v", response.Errors)
	}

	if jsonResponse == nil {
		return errs.New("empty response: %q", b)
	}

	return json.NewDecoder(bytes.NewReader(response.Data)).Decode(jsonResponse)
}

func (ce *consoleEndpoints) createOrGetAPIKey() (string, error) {
	authToken, err := ce.tryLogin()
	if err != nil {
		_ = ce.tryCreateAndActivateUser()
		authToken, err = ce.tryLogin()
		if err != nil {
			return "", errs.Wrap(err)
		}
	}

	err = ce.setupAccount(authToken)
	if err != nil {
		return "", errs.Wrap(err)
	}

	err = ce.addCreditCard(authToken, "test")
	if err != nil {
		return "", errs.Wrap(err)
	}

	cards, err := ce.listCreditCards(authToken)
	if err != nil {
		return "", errs.Wrap(err)
	}
	if len(cards) == 0 {
		return "", errs.New("no credit card(s) found")
	}

	err = ce.makeCreditCardDefault(authToken, cards[0].ID)
	if err != nil {
		return "", errs.Wrap(err)
	}

	projectID, err := ce.getOrCreateProject(authToken)
	if err != nil {
		return "", errs.Wrap(err)
	}

	apiKey, err := ce.createAPIKey(authToken, projectID)
	if err != nil {
		return "", errs.Wrap(err)
	}

	return apiKey, nil
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

	return createTokenResponse.Secret, nil
}

func (ce *consoleEndpoints) createUser(regToken string) (string, error) {
	var registerData struct {
		FullName  string `json:"fullName"`
		ShortName string `json:"shortName"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Secret    string `json:"secret"`
	}

	registerData.FullName = "Alice"
	registerData.Email = "alice@mail.test"
	registerData.Password = "123a123"
	registerData.ShortName = "al"
	registerData.Secret = regToken

	res, err := json.Marshal(registerData)
	if err != nil {
		return "", errs.Wrap(err)
	}

	request, err := http.NewRequest(
		http.MethodPost,
		ce.Register(),
		bytes.NewReader(res))
	if err != nil {
		return "", errs.Wrap(err)
	}
	request.Header.Add("Content-Type", "application/json")

	resp, err := ce.client.Do(request)
	if err != nil {
		return "", errs.Wrap(err)
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

	activationToken, err := generateActivationKey(userUUID, "alice@mail.test", time.Now())
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

	resp, err := ce.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

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
	if err := ce.graphqlDo(request, &getProjects); err != nil {
		return "", errs.Wrap(err)
	}
	if len(getProjects.MyProjects) == 0 {
		return "", errs.New("no projects")
	}

	return getProjects.MyProjects[0].ID, nil
}

func (ce *consoleEndpoints) createProject(token string) (string, error) {
	rng := rand.NewSource(time.Now().UnixNano())
	createProjectQuery := fmt.Sprintf(
		`mutation {createProject(input:{name:"TestProject-%d",description:""}){id}}`,
		rng.Int63())

	request, err := http.NewRequest(
		http.MethodPost,
		ce.GraphQL(),
		bytes.NewReader([]byte(createProjectQuery)))
	if err != nil {
		return "", errs.Wrap(err)
	}

	request.AddCookie(&http.Cookie{
		Name:  ce.cookieName,
		Value: token,
	})

	request.Header.Add("Content-Type", "application/graphql")

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

	request, err := http.NewRequest(
		http.MethodPost,
		ce.GraphQL(),
		bytes.NewReader([]byte(createAPIKeyQuery)))
	if err != nil {
		return "", errs.Wrap(err)
	}

	request.AddCookie(&http.Cookie{
		Name:  ce.cookieName,
		Value: token,
	})

	request.Header.Add("Content-Type", "application/graphql")

	var createAPIKey struct {
		CreateAPIKey struct {
			Key string
		}
	}
	if err := ce.graphqlDo(request, &createAPIKey); err != nil {
		return "", errs.Wrap(err)
	}

		request, err := http.NewRequest(
			http.MethodPost,
			endpoints["graphql"],
			bytes.NewReader([]byte(createAPIKeyQuery)))
		if err != nil {
			return err
		}

		request.Header.Add("Content-Type", "application/graphql")
		request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	resJSON, err := claims.JSON()
	if err != nil {
		return "", err
	}

	token := consoleauth.Token{Payload: resJSON}
	encoded := base64.URLEncoding.EncodeToString(token.Payload)

	signature, err := signer.Sign([]byte(encoded))
	if err != nil {
		return "", err
	}

	token.Signature = signature

	return token.String(), nil
}

func tryReadLine(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Text()
}
