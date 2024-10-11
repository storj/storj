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
)

// EmulatorAdmin provides facilities to communicate with the Spanner Emulator to create
// new instances and databases.
type EmulatorAdmin struct {
	Params    ConnParams
	Instances *instance.InstanceAdminClient
	Databases *database.DatabaseAdminClient
}

// OpenEmulatorAdmin creates a new emulator admin that uses the specified endpoint.
func OpenEmulatorAdmin(params ConnParams) *EmulatorAdmin {
	return &EmulatorAdmin{Params: params}
}

func (admin *EmulatorAdmin) ensureInstances(ctx context.Context) error {
	instanceClient, err := instance.NewInstanceAdminClient(ctx, admin.Params.ClientOptions()...)
	if err != nil {
		return fmt.Errorf("failed to create instance admin: %w", err)
	}
	admin.Instances = instanceClient
	return nil
}

func (admin *EmulatorAdmin) ensureDatabases(ctx context.Context) error {
	databaseClient, err := database.NewDatabaseAdminClient(ctx, admin.Params.ClientOptions()...)
	if err != nil {
		return fmt.Errorf("failed to create database admin: %w", err)
	}
	admin.Databases = databaseClient
	return nil
}

// Close closes the underlying clients.
func (admin *EmulatorAdmin) Close() error {
	var errInstances, errDatabases error
	if admin.Instances != nil {
		errInstances = admin.Instances.Close()
	}
	if admin.Databases != nil {
		errDatabases = admin.Databases.Close()
	}
	return errors.Join(errInstances, errDatabases)
}

// CreateInstance creates a new instance with the specified name.
func (admin *EmulatorAdmin) CreateInstance(ctx context.Context, params ConnParams) error {
	if params.Project == "" || params.Instance == "" {
		return errors.New("project and instance are required")
	}
	if err := admin.ensureInstances(ctx); err != nil {
		return err
	}

	op, err := admin.Instances.CreateInstance(ctx, &instancepb.CreateInstanceRequest{
		Parent:     params.ProjectPath(),
		InstanceId: params.Instance,
		Instance: &instancepb.Instance{
			Config:      params.ProjectPath() + "/instanceConfigs/emulator-config",
			DisplayName: params.Instance,
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
func (admin *EmulatorAdmin) DeleteInstance(ctx context.Context, params ConnParams) error {
	if params.Project == "" || params.Instance == "" {
		return errors.New("project and instance are required")
	}
	if err := admin.ensureInstances(ctx); err != nil {
		return err
	}

	err := admin.Instances.DeleteInstance(ctx, &instancepb.DeleteInstanceRequest{
		Name: params.InstancePath(),
	})
	if err != nil {
		return fmt.Errorf("failed DeleteInstance: %w", err)
	}
	return nil
}

// CreateDatabase creates a new database with the specified name.
func (admin *EmulatorAdmin) CreateDatabase(ctx context.Context, params ConnParams, ddls ...string) error {
	if params.Project == "" || params.Instance == "" || params.Database == "" {
		return errors.New("project, instance and database are required")
	}
	if err := admin.ensureDatabases(ctx); err != nil {
		return err
	}

	op, err := admin.Databases.CreateDatabase(ctx, &databasepb.CreateDatabaseRequest{
		Parent:          params.InstancePath(),
		CreateStatement: "CREATE DATABASE `" + params.Database + "`",
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
func (admin *EmulatorAdmin) DropDatabase(ctx context.Context, params ConnParams) error {
	if params.Project == "" || params.Instance == "" || params.Database == "" {
		return errors.New("project, instance and database are required")
	}
	if err := admin.ensureDatabases(ctx); err != nil {
		return err
	}

	err := admin.Databases.DropDatabase(ctx, &databasepb.DropDatabaseRequest{
		Database: params.DatabasePath(),
	})
	if err != nil {
		return fmt.Errorf("failed DropDatabase: %w", err)
	}
	return nil
}

// DialDatabase creates a new connection to the spanner instance.
func DialDatabase(ctx context.Context, params ConnParams) (*spanner.Client, error) {
	if params.Project == "" || params.Instance == "" || params.Database == "" {
		return nil, errors.New("project, instance and database are required")
	}

	return spanner.NewClientWithConfig(ctx,
		params.DatabasePath(),
		spanner.ClientConfig{
			DisableRouteToLeader: true,
		},
		params.ClientOptions()...,
	)
}
