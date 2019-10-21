// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { CreateProjectModel, Project, ProjectsApi } from '@/types/projects';

/**
 * Mock for ProjectsApi
 */
export class ProjectsApiMock implements ProjectsApi {
    private mockProjects: Project[];

    public setMockProjects(mockProjects: Project[]): void {
        this.mockProjects = mockProjects;
    }

    create(createProjectModel: CreateProjectModel): Promise<Project> {
        throw new Error('not implemented');
    }

    delete(projectId: string): Promise<void> {
        throw new Error('not implemented');
    }

    get(): Promise<Project[]> {
        return Promise.resolve(this.mockProjects);
    }

    update(projectId: string, description: string): Promise<void> {
        throw new Error('not implemented');
    }
}
