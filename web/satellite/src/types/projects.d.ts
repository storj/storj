// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Project is a type, used for creating new project in backend
declare type Project = {
    id: string,

    name: string,
    description: string,
    createdAt: string,

    isSelected: boolean,
};

// UpdateProjectModel is a type, used for updating project description
declare type UpdateProjectModel = {
    id: string,
    description: string,
};

// CreateProjectModel is a type, used for creating project
declare type CreateProjectModel = {
    name: string,
    description: string,
}
