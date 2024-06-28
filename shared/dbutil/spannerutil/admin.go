// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
	databasepb "cloud.google.com/go/spanner/admin/database/apiv1/databasepb"
	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	instancepb "cloud.google.com/go/spanner/admin/instance/apiv1/instancepb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// EmulatorAdmin provides facilities to communicate with the Spanner Emulator to create
// new instances and databases.
type EmulatorAdmin struct {
	HostPort  string
	Instances *instance.InstanceAdminClient
	Databases *database.DatabaseAdminClient
}

// OpenEmulatorAdmin creates a new emulator admin that uses the specified endpoint.
func OpenEmulatorAdmin(ctx context.Context, hostport string) (*EmulatorAdmin, error) {
	options := []option.ClientOption{
		option.WithEndpoint(hostport),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithoutAuthentication(),
	}

	instanceClient, err := instance.NewInstanceAdminClient(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create instance admin: %w", err)
	}

	databaseClient, err := database.NewDatabaseAdminClient(ctx, options...)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("failed to create database admin: %w", err), instanceClient.Close())
	}

	return &EmulatorAdmin{
		HostPort:  hostport,
		Instances: instanceClient,
		Databases: databaseClient,
	}, nil
}

// CreateInstance creates a new instance with the specified name.
func (admin *EmulatorAdmin) CreateInstance(ctx context.Context, projectID, instanceID string) error {
	op, err := admin.Instances.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     "projects/" + projectID,
		InstanceId: instanceID,
		Instance: &instancepb.Instance{
			Config:      "projects/" + projectID + "/instanceConfigs/emulator-config",
			DisplayName: instanceID,
			NodeCount:   1,
		},
	})
	if err != nil {
		return fmt.Errorf("failed CreateInstance: %w", err)
	}

	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("failed CreateInstance.Wait: %w", err)
	}

	return nil
}

// DeleteInstance deletes an instance with the specified name.
func (admin *EmulatorAdmin) DeleteInstance(ctx context.Context, projectID, instanceID string) error {
	err := admin.Instances.DeleteInstance(ctx, &instancepb.DeleteInstanceRequest{
		Name: "projects/" + projectID + "/instances/" + instanceID,
	})
	if err != nil {
		return fmt.Errorf("failed DeleteInstance: %w", err)
	}
	return nil
}

// CreateDatabase creates a new database with the specified name.
func (admin *EmulatorAdmin) CreateDatabase(ctx context.Context, projectID, instanceID, databaseID string, ddls ...string) error {
	op, err := admin.Databases.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          "projects/" + projectID + "/instances/" + instanceID,
		CreateStatement: "CREATE DATABASE `" + databaseID + "`",
		ExtraStatements: ddls,
	})
	if err != nil {
		return fmt.Errorf("failed CreateDatabase: %w", err)
	}
	if _, err := op.Wait(ctx); err != nil {
		return fmt.Errorf("failed CreateDatabase.Wait: %w", err)
	}
	return nil
}

// DropDatabase deletes the specified database.
func (admin *EmulatorAdmin) DropDatabase(ctx context.Context, projectID, instanceID, databaseID string) error {
	err := admin.Databases.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: "projects/" + projectID + "/instances/" + instanceID + "/databases/" + databaseID,
	})
	if err != nil {
		return fmt.Errorf("failed DropDatabase: %w", err)
	}
	return nil
}

// DialDatabase creates a new connection to the spanner instance.
func (admin *EmulatorAdmin) DialDatabase(ctx context.Context, projectID, instanceID, databaseID string) (*spanner.Client, error) {
	return spanner.NewClientWithConfig(ctx,
		"projects/"+projectID+"/instances/"+instanceID+"/databases/"+databaseID,
		spanner.ClientConfig{
			DisableRouteToLeader: true,
		},
		option.WithEndpoint(admin.HostPort),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		option.WithoutAuthentication(),
	)
}

// Close closes the underlying clients.
func (admin *EmulatorAdmin) Close() error {
	return errors.Join(admin.Instances.Close(), admin.Databases.Close())
}
