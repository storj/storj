// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package durability

import "storj.io/storj/satellite/nodeselection"

// Config contains configuration for the durability ranged loop observer.
type Config struct {
	Classes []string `help:"Node attributes used by the durability segment loop to classify risks" default:"last_net,last_ip,wallet,email" testDefault:""`
}

// CreateNodeClassifiers creates a list of node classifiers based on the configuration.
func (c Config) CreateNodeClassifiers() (map[string]NodeClassifier, error) {
	classifiers := map[string]NodeClassifier{}
	for _, class := range c.Classes {
		classifier, err := nodeselection.CreateNodeAttribute(class)
		if err != nil {
			return nil, err
		}
		classifiers[class] = func(node *nodeselection.SelectedNode) string {
			return classifier(*node)
		}
	}
	return classifiers, nil
}
