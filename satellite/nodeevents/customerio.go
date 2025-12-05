// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
)

// CustomerioConfig handles customer.io credentials info.
type CustomerioConfig struct {
	URL            string        `help:"the url for the customer.io endpoint to send node event data to" default:"https://track.customer.io/api/v1"`
	SiteID         string        `help:"the account id for the customer.io api" default:""`
	APIKey         string        `help:"api key for the customer.io api" default:""`
	RequestTimeout time.Duration `help:"timeout for the http request to customer.io endpoint" default:"30s"`
}

// CustomerioNotifier notifies customer.io about node events.
type CustomerioNotifier struct {
	log    *zap.Logger
	config CustomerioConfig
	client *http.Client
}

// CustomerioBatch contains info regarding a batch of node events
// for a particular node operator email address.
type CustomerioBatch struct {
	Name string         `json:"name"`
	Data CustomerioData `json:"data"`
}

// CustomerioData contains the satellite name and the node IDs that had an occurrence of the event.
type CustomerioData struct {
	Satellite string `json:"satellite"`
	NodeIDs   string `json:"nodeIDs"`
	IPPorts   string `json:"ipPorts"`
}

// NewCustomerioNotifier is a constructor for CustomerioNotifier.
func NewCustomerioNotifier(log *zap.Logger, config CustomerioConfig) *CustomerioNotifier {
	return &CustomerioNotifier{
		log:    log,
		config: config,
		client: &http.Client{
			Timeout: config.RequestTimeout,
		},
	}
}

// Notify sends node event data to customer.io.
func (c *CustomerioNotifier) Notify(ctx context.Context, satellite string, events []NodeEvent) (err error) {
	defer mon.Task()(&ctx)(&err)

	if len(events) == 0 {
		return nil
	}

	email := events[0].Email
	eventName, err := events[0].Event.Name()
	if err != nil {
		return err
	}

	var nodeIDs, ipPorts string
	seen := make(map[storj.NodeID]struct{})
	for _, e := range events {
		if _, ok := seen[e.NodeID]; !ok {
			seen[e.NodeID] = struct{}{}
			nodeIDs = nodeIDs + e.NodeID.String() + ","
			if e.LastIPPort != nil {
				ipPorts = ipPorts + *e.LastIPPort + ","
			}
		}
	}
	nodeIDs = strings.TrimSuffix(nodeIDs, ",")
	ipPorts = strings.TrimSuffix(ipPorts, ",")

	batch := CustomerioBatch{
		Name: eventName,
		Data: CustomerioData{
			Satellite: satellite,
			NodeIDs:   nodeIDs,
			IPPorts:   ipPorts,
		},
	}
	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	url := c.config.URL + "/customers/" + email + "/events"

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(data),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(c.config.SiteID, c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	c.log.Info("batch sent to customer.io", zap.String("email", email), zap.String("event", eventName), zap.String("node IDs", nodeIDs))

	if resp.StatusCode != http.StatusOK {
		return errs.New("unexpected status code: %d", resp.StatusCode)
	}
	return err

}
