// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-oauth2/oauth2/v4/errors"

	"storj.io/storj/metasearch"
)

// MetaSearchClient proides a client for the metasearch REST service.
type MetaSearchClient struct {
	access *AccessOptions
	client *http.Client
}

func newMetaSearchClient(access *AccessOptions) *MetaSearchClient {
	client := &http.Client{}
	return &MetaSearchClient{
		access: access,
		client: client,
	}
}

// GetObjectMetadata retrieves the metadata for an object.
func (c *MetaSearchClient) GetObjectMetadata(ctx context.Context, bucket string, key string) (meta map[string]interface{}, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.access.Server+"/metadata/"+bucket+"/"+key, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.access.Access)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, httpError(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&meta)
	if err != nil {
		return nil, fmt.Errorf("cannot decode metadata: %w", err)
	}

	return meta, nil
}

// SetObjectMetadata sets the metadata for an object.
func (c *MetaSearchClient) SetObjectMetadata(ctx context.Context, bucket string, key string, metadata map[string]interface{}) error {
	buf, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("cannot encode metadata: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.access.Server+"/metadata/"+bucket+"/"+key, bytes.NewReader(buf))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.access.Access)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return httpError(resp)
	}

	return nil
}

// DeleteObjectMetadata deletes the metadata for an object.
func (c *MetaSearchClient) DeleteObjectMetadata(ctx context.Context, bucket string, key string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.access.Server+"/metadata/"+bucket+"/"+key, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.access.Access)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return httpError(resp)
	}

	return nil
}

func (c *MetaSearchClient) SearchMetadata(ctx context.Context, bucket string, prefix string, match map[string]interface{}, filter string, projection string, pageToken string) (result metasearch.SearchResponse, err error) {
	body := metasearch.SearchRequest{
		KeyPrefix:  prefix,
		Match:      match,
		Filter:     filter,
		Projection: projection,
		PageToken:  pageToken,
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return result, fmt.Errorf("cannot encode search request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.access.Server+"/metasearch/"+bucket, bytes.NewReader(buf))
	if err != nil {
		return result, fmt.Errorf("cannot create search request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.access.Access)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result, httpError(resp)
	}

	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return result, fmt.Errorf("cannot decode search response: %w", err)
	}

	return result, nil
}

func httpError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return errors.New("unauthorized")
	case http.StatusNotFound:
		return errors.New("object not found")
	default:
		return fmt.Errorf("error response from server: %v", resp.Status)
	}
}
