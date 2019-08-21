// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all user-related functionality
 */
export interface ProjectsApi {
    /**
     * Updates users full name and short name
     *
     * @param createProjectModel - contains project information
     * @throws Error
     */
    create(createProjectModel: CreateProjectModel): Promise<Project>;
    /**
     * Fetch projects
     *
     * @returns Project[]
     * @throws Error
     */
    get(): Promise<Project[]>;
    /**
     * Update user
     *
     * @param projectId - project ID
     * @param description - project description
     * @returns Project[]
     * @throws Error
     */
    update(projectId: string, description: string): Promise<void>;
    /**
     * Delete project
     *
     * @param projectId - project ID
     * @throws Error
     */
    delete(projectId: string): Promise<void>;
}

// Project is a type, used for creating new project in backend
export class Project {
    public id: string;

    public name: string;
    public description: string;
    public createdAt: string;

    public isSelected: boolean;

    public constructor(id: string = '', name: string = '', description: string = '', createdAt: string = '', isSelected:boolean = false) {
        this.id = id;
        this.name = name;
        this.description = description;
        this.createdAt = createdAt;
        this.isSelected = isSelected;
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
