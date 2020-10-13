// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { Project, ProjectFields, ProjectLimits, ProjectsApi } from '@/types/projects';
import { HttpClient } from '@/utils/httpClient';

export class ProjectsApiGql extends BaseGql implements ProjectsApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/projects';

    /**
     * Creates project.
     *
     * @param projectFields - contains project information
     * @throws Error
     */
    public async create(projectFields: ProjectFields): Promise<Project> {
        const query =
            `mutation($name: String!, $description: String!) {
                createProject(
                    input: {
                        name: $name,
                        description: $description,
                    }
                ) {id}
            }`;

        const variables = {
            name: projectFields.name,
            description: projectFields.description,
        };

        const response = await this.mutate(query, variables);

        return new Project(response.data.createProject.id, variables.name, variables.description, '', projectFields.ownerId);
    }

    /**
     * Fetch projects.
     *
     * @returns Project[]
     * @throws Error
     */
    public async get(): Promise<Project[]> {
        const query = `query {
            myProjects{
                name
                id
                description
                createdAt
                ownerId
            }
        }`;

        const response = await this.query(query);

        return response.data.myProjects.map((project: Project) => {
            return new Project(
                project.id,
                project.name,
                project.description,
                project.createdAt,
                project.ownerId,
            );
        });
    }

    /**
     * Update project name and description.
     *
     * @param projectId - project ID
     * @param name - project name
     * @param description - project description
     * @returns Project[]
     * @throws Error
     */
    public async update(projectId: string, name: string, description: string): Promise<void> {
        const query =
            `mutation($projectId: String!, $name: String!, $description: String!) {
                updateProject(
                    id: $projectId,
                    name: $name,
                    description: $description
                ) {name}
            }`;

        const variables = {
            projectId: projectId,
            name: name,
            description: description,
        };

        await this.mutate(query, variables);
    }

    /**
     * Delete project.
     *
     * @param projectId - project ID
     * @throws Error
     */
    public async delete(projectId: string): Promise<void> {
        const query =
            `mutation($projectId: String!) {
                deleteProject(
                    id: $projectId
                ) {name}
            }`;

        const variables = {
            projectId: projectId,
        };

        await this.mutate(query, variables);
    }

    /**
     * Get project limits.
     *
     * @param projectId- project ID
     * throws Error
     */
    public async getLimits(projectId): Promise<ProjectLimits> {
        const path = `${this.ROOT_PATH}/${projectId}/usage-limits`;
        const response = await this.http.get(path);

        if (response.ok) {
            const limits = await response.json();

            return new ProjectLimits(
                limits.bandwidthLimit,
                limits.bandwidthUsed,
                limits.storageLimit,
                limits.storageUsed,
            );
        }

        if (response.status === 401) {
            throw new ErrorUnauthorized();
        }

        throw new Error('can not get usage limits');
    }
}
