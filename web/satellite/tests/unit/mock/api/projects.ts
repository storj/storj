// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Project, ProjectFields, ProjectLimits, ProjectsApi, ProjectsCursor, ProjectsPage } from '@/types/projects';

/**
 * Mock for ProjectsApi
 */
export class ProjectsApiMock implements ProjectsApi {
    private mockProjects: Project[];
    private mockLimits: ProjectLimits;
    private mockProjectsPage: ProjectsPage;

    public setMockProjects(mockProjects: Project[]): void {
        this.mockProjects = mockProjects;
    }

    public setMockLimits(mockLimits: ProjectLimits): void {
        this.mockLimits = mockLimits;
    }

    create(createProjectFields: ProjectFields): Promise<Project> {
        throw new Error('not implemented');
    }

    delete(projectId: string): Promise<void> {
        throw new Error('not implemented');
    }

    get(): Promise<Project[]> {
        return Promise.resolve(this.mockProjects);
    }

    getOwnedProjects(cursor: ProjectsCursor): Promise<ProjectsPage> {
        return Promise.resolve(this.mockProjectsPage);
    }

    update(projectId: string, name: string, description: string): Promise<void> {
        return Promise.resolve();
    }

    getLimits(projectId: string): Promise<ProjectLimits> {
        return Promise.resolve(this.mockLimits);
    }
}
