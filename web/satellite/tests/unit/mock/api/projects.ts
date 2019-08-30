// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ProjectsApi, Project, CreateProjectModel } from '@/types/projects';

/**
 * Mock for CreditsApi
 */
export class ProjectsApiMock implements ProjectsApi {
    private mockProject: Project;

    public setMockProject(mockCredits: Project): void {
        this.mockProject = mockCredits;
    }

    create(createProjectModel: CreateProjectModel): Promise<Project> {
        throw new Error('not implemented');
    }

    delete(projectId: string): Promise<void> {
        throw new Error('not implemented');
    }

    get(): Promise<Project[]> {
        const result = Array<Project>();
        result.push(this.mockProject);

        return Promise.resolve(result);
    }

    update(projectId: string, description: string): Promise<void> {
        throw new Error('not implemented');
    }
}
