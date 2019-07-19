// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Project is a type, used for creating new project in backend
export class Project {
    public id: string = '';

    public name: string = '';
    public description: string = '';
    public createdAt: string = '';

    public isSelected: boolean = false;

    public constructor(isSelected?:boolean) {
        this.isSelected = isSelected || false;
    }
}

// UpdateProjectModel is a type, used for updating project description
export class UpdateProjectModel {
    public id: string;
    public description: string;
}

// CreateProjectModel is a type, used for creating project
export class CreateProjectModel {
    public name: string;
    public description: string;
}
