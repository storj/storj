// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all project-related functionality.
 */
export interface ProjectsApi {
    /**
     * Creates project.
     *
     * @param createProjectModel - contains project information
     * @throws Error
     */
    create(createProjectModel: CreateProjectModel): Promise<Project>;
    /**
     * Fetch projects.
     *
     * @returns Project[]
     * @throws Error
     */
    get(): Promise<Project[]>;
    /**
     * Update project.
     *
     * @param projectId - project ID
     * @param description - project description
     * @returns Project[]
     * @throws Error
     */
    update(projectId: string, description: string): Promise<void>;
    /**
     * Delete project.
     *
     * @param projectId - project ID
     * @throws Error
     */
    delete(projectId: string): Promise<void>;

    /**
     * Get project limits.
     *
     * @param projectId- project ID
     * throws Error
     */
    getLimits(projectId: string): Promise<ProjectLimits>;
}

/**
 * Project is a type, used for creating new project in backend.
 */
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

/**
 * UpdateProjectModel is a type, used for updating project description.
 */
export class UpdateProjectModel {
    public id: string;
    public description: string;

    public constructor(id: string, description: string) {
        this.id = id;
        this.description = description;
    }
}

/**
 * CreateProjectModel is a type, used for creating project.
 */
export class CreateProjectModel {
    private readonly MAX_NAME_LENGTH = 20;

    public constructor(
        public name: string = '',
        public description: string = '',
        public ownerId: string = '',
    ) {}

    /**
     * checkName checks if project name is valid.
     */
    public checkName(): void {
        try {
            this.nameIsNotEmpty();
            this.nameHasNoSlashes();
            this.nameHasLessThenTwentySymbols();
        } catch (error) {
            throw new Error(error.message);
        }
    }

    /**
     * nameHasNoSlashes checks if project name has any characters but 'slash'.
     */
    private nameHasNoSlashes(): void {
        const rgx = /^[^\/]+$/;

        if (!rgx.test(this.name)) throw new Error('Project name can\'t have slashes!');
    }

    /**
     * nameIsNotEmpty checks if project name is not empty.
     */
    private nameIsNotEmpty(): void {
        if (this.name.length === 0) throw new Error('Project name can\'t be empty!');
    }

    /**
     * nameHasLessThenTwentySymbols checks if project name has less then 20 symbols.
     */
    private nameHasLessThenTwentySymbols(): void {
        if (this.name.length > this.MAX_NAME_LENGTH) throw new Error('Name should be less than 21 character!');
    }
}

/**
 * ProjectLimits is a type, used for describing project limits.
 */
export class ProjectLimits {
    public constructor(
        public bandwidthLimit: number = 0,
        public bandwidthUsed: number = 0,
        public storageLimit: number = 0,
        public storageUsed: number = 0,
    ) {}
}
