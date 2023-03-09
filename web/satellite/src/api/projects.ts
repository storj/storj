// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import {
    DataStamp,
    Project,
    ProjectFields,
    ProjectLimits,
    ProjectsApi,
    ProjectsCursor,
    ProjectsPage,
    ProjectsStorageBandwidthDaily,
} from '@/types/projects';
import { HttpClient } from '@/utils/httpClient';
import { Time } from '@/utils/time';

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
                ) {publicId}
            }`;

        const variables = {
            name: projectFields.name,
            description: projectFields.description,
        };

        const response = await this.mutate(query, variables);

        return new Project(response.data.createProject.publicId, variables.name, variables.description, '', projectFields.ownerId);
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
                publicId
                description
                createdAt
                ownerId
            }
        }`;

        const response = await this.query(query);

        return response.data.myProjects.map((project: Project & {publicId: string}) => {
            return new Project(
                project.publicId,
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
     * @param projectFields - project fields
     * @param projectLimits - project limits
     * @returns Project[]
     * @throws Error
     */
    public async update(projectId: string, projectFields: ProjectFields, projectLimits: ProjectLimits): Promise<void> {
        const query =
            `mutation($projectId: String!, $name: String!, $description: String!, $storageLimit: String!, $bandwidthLimit: String!) {
                updateProject(
                    publicId: $projectId,
                    projectFields: {
                        name: $name,
                        description: $description,
                    },
                    projectLimits: {
                        storageLimit: $storageLimit,
                        bandwidthLimit: $bandwidthLimit,
                    }
                ) {name}
            }`;

        const variables = {
            projectId: projectId,
            name: projectFields.name,
            description: projectFields.description,
            storageLimit: projectLimits.storageLimit.toString(),
            bandwidthLimit: projectLimits.bandwidthLimit.toString(),
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
                    publicId: $projectId
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
    public async getLimits(projectId: string): Promise<ProjectLimits> {
        const path = `${this.ROOT_PATH}/${projectId}/usage-limits`;
        const response = await this.http.get(path);

        if (!response.ok) {
            throw new Error('can not get usage limits');
        }

        const limits = await response.json();

        return new ProjectLimits(
            limits.bandwidthLimit,
            limits.bandwidthUsed,
            limits.storageLimit,
            limits.storageUsed,
            limits.objectCount,
            limits.segmentCount,
            limits.segmentLimit,
            limits.segmentUsed,
        );

    }

    /**
     * Get total limits for all the projects that user owns.
     *
     * throws Error
     */
    public async getTotalLimits(): Promise<ProjectLimits> {
        const path = `${this.ROOT_PATH}/usage-limits`;
        const response = await this.http.get(path);

        if (!response.ok) {
            throw new Error('can not get total usage limits');

        }

        const limits = await response.json();

        return new ProjectLimits(
            limits.bandwidthLimit,
            limits.bandwidthUsed,
            limits.storageLimit,
            limits.storageUsed,
        );
    }

    /**
     * Get project daily usage for specific date range.
     *
     * @param projectId- project ID
     * @param start- since date
     * @param end- before date
     * throws Error
     */
    public async getDailyUsage(projectId: string, start: Date, end: Date): Promise<ProjectsStorageBandwidthDaily> {
        const since = Time.toUnixTimestamp(start).toString();
        const before = Time.toUnixTimestamp(end).toString();
        const path = `${this.ROOT_PATH}/${projectId}/daily-usage?from=${since}&to=${before}`;
        const response = await this.http.get(path);

        if (!response.ok) {
            throw new Error('Can not get project daily usage');

        }

        const usage = await response.json();

        return new ProjectsStorageBandwidthDaily(
            usage.storageUsage.map(el => {
                const date = new Date(el.date);
                date.setHours(0, 0, 0, 0);
                return new DataStamp(el.value, date);
            }),
            usage.allocatedBandwidthUsage.map(el => {
                const date = new Date(el.date);
                date.setHours(0, 0, 0, 0);
                return new DataStamp(el.value, date);
            }),
            usage.settledBandwidthUsage.map(el => {
                const date = new Date(el.date);
                date.setHours(0, 0, 0, 0);
                return new DataStamp(el.value, date);
            }),
        );
    }

    public async getSalt(projectId: string): Promise<string> {
        const path = `${this.ROOT_PATH}/${projectId}/salt`;
        const response = await this.http.get(path);
        if (response.ok) {
            return await response.json();
        }

        throw new Error('Can not get project salt');
    }

    /**
     * Fetch owned projects.
     *
     * @returns ProjectsPage
     * @throws Error
     */
    public async getOwnedProjects(cursor: ProjectsCursor): Promise<ProjectsPage> {
        const query =
            `query($limit: Int!, $page: Int!) {
                ownedProjects( cursor: { limit: $limit, page: $page } ) {
                    projects {
                        publicId,
                        name,
                        ownerId,
                        description,
                        createdAt,
                        memberCount
                    },
                    limit,
                    offset,
                    pageCount,
                    currentPage,
                    totalCount
                 }
             }`;

        const variables = {
            limit: cursor.limit,
            page: cursor.page,
        };

        const response = await this.query(query, variables);

        return this.getProjectsPage(response.data.ownedProjects);
    }

    /**
     * Method for mapping projects page from json to ProjectsPage type.
     *
     * @param page anonymous object from json
     */
    private getProjectsPage(page: any): ProjectsPage { // eslint-disable-line @typescript-eslint/no-explicit-any
        if (!page) {
            return new ProjectsPage();
        }

        const projects: Project[] = page.projects.map(key =>
            new Project(
                key.publicId,
                key.name,
                key.description,
                key.createdAt,
                key.ownerId,
                false,
                key.memberCount));

        return new ProjectsPage(projects, page.limit, page.offset, page.pageCount, page.currentPage, page.totalCount);
    }

}
