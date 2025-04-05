// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package tracker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// PrometheusTrackerConfig is the configuration for the PrometheusTracker.
type PrometheusTrackerConfig struct {
	URL        string   `help:"URL of the Prometheus server"`
	CaCertPath string   `help:"Path to the CA certificate of the Prometheus server"`
	Username   string   `help:"Username for the basic auth of the Prometheus server"`
	Password   string   `help:"Password for the basic auth of the Prometheus server"`
	Query      string   `help:"Prometheus query to get the scores"`
	Attributes []string `help:"Node attributes to use for matching nodes with metrics"`
	Labels     []string `help:"Labels to use for matching nodes with metrics"`
}

type prometheusScoreCacheState map[string]float64

type prometheusNodeCacheState map[storj.NodeID]string

// PrometheusTracker is a tracker (ScoreNode implementation) which scores the nodes based on metrics from external Prometheus server.
// As the metrics may not be tagged with node_id, the tracker pairs all the metrics with nodes based on attributes / labels.
type PrometheusTracker struct {
	scoreCache sync2.ReadCacheOf[prometheusScoreCacheState]
	nodeCache  sync2.ReadCacheOf[prometheusNodeCacheState]
	log        *zap.Logger
	client     v1.API
	query      string
	overlay    overlay.DB

	// how to create a cache key from a node
	attribute nodeselection.NodeAttribute

	// how to create a cache key from a prometheus metric
	labels []string
}

// NewPrometheusTracker creates a new PrometheusTracker.
func NewPrometheusTracker(log *zap.Logger, db overlay.DB, config PrometheusTrackerConfig) (*PrometheusTracker, error) {
	var attributes []nodeselection.NodeAttribute
	for _, attr := range config.Attributes {
		attribute, err := nodeselection.CreateNodeAttribute(attr)
		if err != nil {
			return nil, errs.New("invalud node attribute for prometheus tracker: %s, %v", attr, err)
		}
		attributes = append(attributes, attribute)
	}

	if len(attributes) == 0 {
		return nil, errs.New("no attributes specified for prometheus tracker")
	}

	if len(attributes) != len(config.Labels) {
		return nil, errs.New("number of attributes and labels must match")
	}

	roundTripper, err := NewSecureRoundTripper(config.CaCertPath, config.Username, config.Password)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	pc, err := api.NewClient(api.Config{
		Address:      config.URL,
		RoundTripper: roundTripper,
	})
	if err != nil {
		return nil, errs.Wrap(err)
	}

	tracker := &PrometheusTracker{
		log:       log,
		client:    v1.NewAPI(pc),
		query:     config.Query,
		overlay:   db,
		attribute: nodeselection.NodeAttributes(attributes, ","),
		labels:    config.Labels,
	}
	err = tracker.scoreCache.Init(3*time.Second, 10*time.Second, tracker.refreshScores)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	err = tracker.nodeCache.Init(3*time.Second, 10*time.Second, tracker.refreshNodes)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return tracker, nil
}

// Run starts the background process for the tracker.
func (p *PrometheusTracker) Run(ctx context.Context) error {
	var eg errgroup.Group
	eg.Go(func() error {
		return p.scoreCache.Run(ctx)
	})
	eg.Go(func() error {
		return p.nodeCache.Run(ctx)
	})
	return eg.Wait()
}

// Get implements ScoreNode.
func (p *PrometheusTracker) Get(uplink storj.NodeID) func(node *nodeselection.SelectedNode) float64 {
	return func(node *nodeselection.SelectedNode) float64 {
		// TODO: it would be nice to make it compatible with the top level CLI context, but nodeselection doesn't support it right now
		ctx, cancel := context.WithTimeout(context.TODO(), 15*time.Second)
		defer cancel()

		scoreState, err := p.scoreCache.RefreshAndGet(ctx, time.Now())
		if err != nil {
			p.log.Warn("Couldn't get the Prometheus data for tracker", zap.Error(err))
			return math.NaN()
		}
		nodeState, err := p.nodeCache.RefreshAndGet(ctx, time.Now())
		if err != nil {
			p.log.Warn("Couldn't get the node data for Prometheus tracker", zap.Error(err))
			return math.NaN()
		}

		key, found := nodeState[node.ID]
		if !found {
			return math.NaN()
		}

		score, found := scoreState[key]
		if !found {
			return math.NaN()
		}

		return score
	}
}

func (p *PrometheusTracker) refreshScores(ctx context.Context) (prometheusScoreCacheState, error) {
	rangeResult, warnings, err := p.client.Query(ctx, p.query, time.Now())
	if err != nil {
		return prometheusScoreCacheState{}, errs.Wrap(err)
	}
	if len(warnings) > 0 {
		p.log.Warn("Warnings during prometheus tracker query", zap.Strings("warnings", warnings))
	}

	state := prometheusScoreCacheState{}

	if v, ok := rangeResult.(model.Vector); ok {
		for _, sample := range v {
			var labelValues []string
			for _, label := range p.labels {
				labelValues = append(labelValues, string(sample.Metric[model.LabelName(label)]))
			}
			state[strings.Join(labelValues, ",")] = float64(sample.Value)
		}
	}
	return state, nil
}

func (p *PrometheusTracker) refreshNodes(ctx context.Context) (prometheusNodeCacheState, error) {
	state := prometheusNodeCacheState{}
	nodes, err := p.overlay.GetAllParticipatingNodes(ctx, 24*time.Hour, -10*time.Millisecond)
	if err != nil {
		return state, errs.Wrap(err)
	}
	for _, node := range nodes {
		key := p.attribute(node)
		if key != "" {
			state[node.ID] = key
		}
	}
	return state, nil
}

var _ nodeselection.ScoreNode = &PrometheusTracker{}

// SecureRoundTripper is a http.RoundTripper that uses a custom CA certificate and basic auth.
type SecureRoundTripper struct {
	Username  string
	Password  string
	transport *http.Transport
}

// NewSecureRoundTripper creates a new SecureRoundTripper.
func NewSecureRoundTripper(caCertPath string, username, password string) (*SecureRoundTripper, error) {
	// Read custom CA certificate
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	// Create cert pool and add our custom CA
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, errs.Wrap(err)
	}

	tlsConfig := &tls.Config{
		RootCAs: caCertPool,
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	return &SecureRoundTripper{
		Username:  username,
		Password:  password,
		transport: transport,
	}, nil
}

// RoundTrip executes a single HTTP transaction.
func (s *SecureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(s.Username, s.Password)
	return s.transport.RoundTrip(req)
}

var _ http.RoundTripper = &SecureRoundTripper{}
