package testidentity

import (
	"context"

	"storj.io/storj/pkg/provider"
)

// helper function to generate new node identities with
// correct difficulty and concurrency
func NewTestIdentity() (*provider.FullIdentity, error) {
	ca, err := provider.NewCA(context.Background(), provider.NewCAOptions{
		Difficulty:  12,
		Concurrency: 4,
	})
	if err != nil {
		return nil, err
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		return nil, err
	}
	return identity, err
}
