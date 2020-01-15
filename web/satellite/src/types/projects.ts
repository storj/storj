// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all project-related functionality
 */
export interface ProjectsApi {
    /**
     * Creates project
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
     * Update project
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

    /**
     * Get project limits
     *
     * @param projectId- project ID
     * throws Error
     */
    getLimits(projectId: string): Promise<ProjectLimits>;
}

// Project is a type, used for creating new project in backend
export class Project {
    public constructor(
        public id: string = '',
        public name: string = '',
        public description: string = '',
        public createdAt: string = '',
        public ownerId: string = '',
        public isSelected: boolean = false,
    ) {}
}

// UpdateProjectModel is a type, used for updating project description
export class UpdateProjectModel {
    public id: string;
    public description: string;

    public constructor(id: string, description: string) {
        this.id = id;
        this.description = description;
    }
}

// CreateProjectModel is a type, used for creating project
export class CreateProjectModel {
    public name: string;
    public description: string;
}

export class ProjectLimits {
    constructor(
        public bandwidthLimit = 0,
        public bandwidthUsed = 0,
        public storageLimit = 0,
        public storageUsed = 0,
    ) {}
}
