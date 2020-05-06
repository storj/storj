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
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console/consoleauth"
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

func (ce *consoleEndpoints) tryLogin() (string, error) {
	var authToken struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	authToken.Email = "alice@mail.test"
	authToken.Password = "123a123"

	res, err := json.Marshal(authToken)
	if err != nil {
		return "", errs.Wrap(err)
	}

	request, err := http.NewRequest(
		http.MethodPost,
		ce.Token(),
		bytes.NewReader(res))
	if err != nil {
		return "", errs.Wrap(err)
	}

	request.Header.Add("Content-Type", "application/json")

	resp, err := ce.client.Do(request)
	if err != nil {
		return "", errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return "", errs.New("unexpected status code: %d (%q)",
			resp.StatusCode, tryReadLine(resp.Body))
	}

	var token string
	err = json.NewDecoder(resp.Body).Decode(&token)
	if err != nil {
		return "", errs.Wrap(err)
	}

	return token, nil
}

func (ce *consoleEndpoints) tryCreateAndActivateUser() error {
	regToken, err := ce.createRegistrationToken()
	if err != nil {
		return errs.Wrap(err)
	}
	userID, err := ce.createUser(regToken)
	if err != nil {
		return errs.Wrap(err)
	}
	return errs.Wrap(ce.activateUser(userID))
}

func (ce *consoleEndpoints) createRegistrationToken() (string, error) {
	request, err := http.NewRequest(
		http.MethodGet,
		ce.RegToken(),
		nil)
	if err != nil {
		return "", errs.Wrap(err)
	}

	resp, err := ce.client.Do(request)
	if err != nil {
		return "", errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return "", errs.New("unexpected status code: %d (%q)",
			resp.StatusCode, tryReadLine(resp.Body))
	}

	var createTokenResponse struct {
		Secret string
		Error  string
	}
	if err = json.NewDecoder(resp.Body).Decode(&createTokenResponse); err != nil {
		return "", errs.Wrap(err)
	}
	if createTokenResponse.Error != "" {
		return "", errs.New("unable to create registration token: %s", createTokenResponse.Error)
	}

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
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return "", errs.New("unexpected status code: %d (%q)",
			resp.StatusCode, tryReadLine(resp.Body))
	}

	var userID string
	if err = json.NewDecoder(resp.Body).Decode(&userID); err != nil {
		return "", errs.Wrap(err)
	}

	return userID, nil
}

func (ce *consoleEndpoints) activateUser(userID string) error {
	userUUID, err := uuid.FromString(userID)
	if err != nil {
		return errs.Wrap(err)
	}

	activationToken, err := generateActivationKey(userUUID, "alice@mail.test", time.Now())
	if err != nil {
		return err
	}

	request, err := http.NewRequest(
		http.MethodGet,
		ce.Activation(activationToken),
		nil)
	if err != nil {
		return err
	}

	resp, err := ce.client.Do(request)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return errs.New("unexpected status code: %d (%q)",
			resp.StatusCode, tryReadLine(resp.Body))
	}

	return nil
}

func (ce *consoleEndpoints) getOrCreateProject(token string) (string, error) {
	projectID, err := ce.getProject(token)
	if err == nil {
		return projectID, nil
	}
	projectID, err = ce.createProject(token)
	if err == nil {
		return projectID, nil
	}
	return ce.getProject(token)
}

func (ce *consoleEndpoints) getProject(token string) (string, error) {
	request, err := http.NewRequest(
		http.MethodGet,
		ce.GraphQL(),
		nil)
	if err != nil {
		return "", errs.Wrap(err)
	}

	q := request.URL.Query()
	q.Add("query", `query {myProjects{id}}`)
	request.URL.RawQuery = q.Encode()

	request.AddCookie(&http.Cookie{
		Name:  ce.cookieName,
		Value: token,
	})

	request.Header.Add("Content-Type", "application/graphql")

	var getProjects struct {
		MyProjects []struct {
			ID string
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
	if err := ce.graphqlDo(request, &createProject); err != nil {
		return "", errs.Wrap(err)
	}

	return createProject.CreateProject.ID, nil
}

func (ce *consoleEndpoints) createAPIKey(token, projectID string) (string, error) {
	rng := rand.NewSource(time.Now().UnixNano())
	createAPIKeyQuery := fmt.Sprintf(
		`mutation {createAPIKey(projectID:%q,name:"TestKey-%d"){key}}`,
		projectID, rng.Int63())

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

	return createAPIKey.CreateAPIKey.Key, nil
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

func tryReadLine(r io.Reader) string {
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Text()
}
