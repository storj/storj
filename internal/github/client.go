package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

const GithubAPIReposURL = "https://api.github.com/repos"
const (
	ReviewApprovalEnum = iota
	ReviewChangesRequestedEnum

	ReviewApproval         = ReviewState(ReviewApprovalEnum)
	ReviewChangesRequested = ReviewState(ReviewChangesRequestedEnum)
)

const (
	MergableStateCleanEnum = iota

	MergableStateClean = MergableState(MergableStateCleanEnum)
)

type ReviewState int
type MergableState int

type ClientConfig struct {
	Repo  string `default:"storj/storj" help:"github user/organization and repository to operate on (i.e. <user/org>/<repo_name>)"`
	User  string `help:"your github username for authentication"`
	Token string `help:"your github oauth token for authentication"`
}

type GithubClient struct {
	config     *ClientConfig
	httpClient *http.Client
	baseUrl    string
}

type JSONResponse http.Response

type PullRequest struct {
	URL    string
	ID     int
	Number int
	Title  string
	User   User
	Labels []Label

	// NB: only on individual PR GET resource
	// TODO: use enum
	MergableState string

	client *GithubClient
}

type Reviews []*Review
type Review struct {
	ID int
	User
	// TODO: use enum
	//State ReviewState
	State string
}

type User struct {
	Login string
	ID    int
}

type Label struct {
	ID   int
	Name string
}

func (jres JSONResponse) UnmarshalJSONBody(v interface{}) error {
	body, err := ioutil.ReadAll(jres.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, v); err != nil {
		return err
	}

	if err := jres.Body.Close(); err != nil {
		return err
	}
	return nil
}

func (gh GithubClient) Request(method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", gh.config.User)
	req.Header.Add("Authorization", gh.config.Token)

	return req, nil
}

func (gh GithubClient) Get(url string) (*JSONResponse, error) {
	req, err := gh.Request("GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := gh.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return (*JSONResponse)(res), nil
}

func (gh *GithubClient) ListPRs() ([]*PullRequest, error) {
	url := fmt.Sprintf("%s/pulls", gh.baseUrl)
	res, err := gh.Get(url)
	if err != nil {
		return nil, err
	}

	var prs []*PullRequest
	if err := res.UnmarshalJSONBody(prs); err != nil {
		return nil, err
	}

	for _, pr := range prs {
		pr.client = gh

		// NB: this could probably be cleaned up by using graphql
		pr, err = pr.Populate()
		if err != nil {
			return nil, err
		}
	}
	return prs, nil
}

func (pr *PullRequest) Populate() (*PullRequest, error) {
	res, err := pr.client.Get(pr.URL)
	if err != nil {
		return nil, err
	}

	if err := res.UnmarshalJSONBody(pr); err != nil {
		return nil, err
	}
	return pr, nil
}

func (pr *PullRequest) Reviews() (Reviews, error) {
	url := fmt.Sprintf("%s/reviews", pr.URL)
	res, err := pr.client.Get(url)
	if err != nil {
		return nil, err
	}

	var reviews Reviews
	if err := res.UnmarshalJSONBody(reviews); err != nil {
		return nil, err
	}

	return reviews, nil
}

func (reviews Reviews) Approvals() int {
	var approvals int
	for _, r := range reviews {
		if r.IsApproval() {
			approvals++
		}
	}
	return approvals
}

func (reviews Reviews) UniqueUsers() int {
	uniqueUsers := make(map[int]*struct{})
	for _, r := range reviews {
		if uniqueUsers[r.User.ID] != nil {
			continue
		}
		uniqueUsers[r.User.ID] = &struct{}{}
	}
	return len(uniqueUsers)
}

func (r Review) IsApproval() bool {
	// TODO: compare enums
	return r.State == ReviewApproval.String()
}

func (r Review) ChangesRequested() bool {
	// TODO: compare enums
	return r.State == ReviewChangesRequested.String()
}

func (rs ReviewState) String() string {
	switch rs {
	case ReviewApprovalEnum:
		return "APPROVED"
	case ReviewChangesRequestedEnum:
		return "CHANGES_REQUESTED"
	default:
		panic(fmt.Sprintf("unknown review state %T", rs))
	}
}

func (ms MergableState) String() string {
	switch ms {
	case MergableStateCleanEnum:
		return "clean"
	default:
		panic(fmt.Sprintf("unknown mergable state %T", ms))
	}
}
