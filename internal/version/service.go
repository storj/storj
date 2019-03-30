// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// Client contains the necessary Information to check the Software Version
type Client struct {
	ServerAddress  string
	RequestTimeout time.Duration
	CheckInterval  time.Duration
	Allowed        bool
}

// NewClient creates a Version Check Client with default configuration
func NewClient() (client *Client) {
	client = &Client{
		ServerAddress:  "https://satellite.stefan-benten.de/version",
		RequestTimeout: time.Second * 30,
		CheckInterval:  time.Minute * 15,
		Allowed:        true,
	}
	return
}

// CheckVersionStartup ensures that client is running latest/allowed code, else refusing further operation
func (cl *Client) CheckVersionStartup(ctx *context.Context) (err error) {
	allow, err := cl.checkVersion(ctx)
	if err == nil {
		cl.Allowed = allow
	}
	zap.S().Debugf("CheckVersionStartup %v", err)
	return
}

// CheckVersion checks if the client is running latest/allowed code
func (cl *Client) checkVersion(ctx *context.Context) (allowed bool, err error) {
	defer mon.Task()(ctx)(&err)
	accepted, err := cl.queryVersionFromControlServer()
	if err != nil {
		return false, err
	}

	zap.S().Debugf("allowed versions from Control Server: %v", accepted)

	// ToDo: Fetch own Service Tag to compare correctly!
	list := accepted.Storagenode
	if list == nil {
		return true, errs.New("Empty List from Versioning Server")
	}
	if containsVersion(list, Build.Version) {
		zap.S().Infof("running on version %s", Build.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("running on not allowed/outdated version %s", Build.Version.String())
		allowed = false
	}
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func (cl *Client) queryVersionFromControlServer() (ver Versions, err error) {
	client := http.Client{
		Timeout: cl.RequestTimeout,
	}
	resp, err := client.Get(cl.ServerAddress)
	if err != nil {
		// ToDo: Make sure Control Server is always reachable and refuse startup
		cl.Allowed = true
		return Versions{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Versions{}, err
	}
	err = json.Unmarshal(body, &ver)
	return
}

// DebugHandler returns a json representation of the current version information for the binary
func (cl *Client) DebugHandler(w http.ResponseWriter, r *http.Request) {
	j, err := Build.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(j)
	if err != nil {
		zap.S().Errorf("error writing data to client %v", err)
	}
}

// LogAndReportVersion logs the current version information
// and reports to monkit
func (cl *Client) LogAndReportVersion(ctx context.Context) (err error) {
	err = cl.CheckVersionStartup(&ctx)
	if err != nil {
		return err
	}
	//Start up periodic checks
	go func(ctx context.Context) {
		ticker := time.NewTicker(cl.CheckInterval)

		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				//Check Version, but dont care if outdated for now
				_, err := cl.checkVersion(&ctx)
				if err != nil {
					zap.S().Errorf("Failed to do periodic version check: ", err)
				}
			}
		}
	}(ctx)
	return
}
