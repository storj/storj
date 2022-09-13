// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    Project,
    ProjectFields,
    ProjectLimits,
    ProjectsApi,
    ProjectsCursor,
    ProjectsPage,
    ProjectsStorageBandwidthDaily,
} from '@/types/projects';

/**
 * Mock for ProjectsApi
 */
export class ProjectsApiMock implements ProjectsApi {
    private mockProjects: Project[] = [];
    private mockLimits: ProjectLimits;
    private mockProjectsPage: ProjectsPage;

    public setMockProjects(mockProjects: Project[]): void {
        this.mockProjects = mockProjects;
    }

    public setMockLimits(mockLimits: ProjectLimits): void {
        this.mockLimits = mockLimits;
    }

    create(_createProjectFields: ProjectFields): Promise<Project> {
        throw new Error('not implemented');
    }

    delete(_projectId: string): Promise<void> {
        throw new Error('not implemented');
    }

    get(): Promise<Project[]> {
        return Promise.resolve(this.mockProjects);
    }

    getOwnedProjects(_cursor: ProjectsCursor): Promise<ProjectsPage> {
        return Promise.resolve(this.mockProjectsPage);
    }

    update(_projectId: string, _projectFields: ProjectFields, _projectLimits: ProjectLimits): Promise<void> {
        return Promise.resolve();
    }

    getLimits(_projectId: string): Promise<ProjectLimits> {
        return Promise.resolve(this.mockLimits);
    }

    getTotalLimits(): Promise<ProjectLimits> {
        return Promise.resolve(this.mockLimits);
    }

    getDailyUsage(_projectId: string, _start: Date, _end: Date): Promise<ProjectsStorageBandwidthDaily> {
        throw new Error('not implemented');
    }
}
