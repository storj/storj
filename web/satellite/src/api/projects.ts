// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import {
    DataStamp,
    Project,
    ProjectFields,
    ProjectInvitation,
    ProjectLimits,
    ProjectsApi,
    ProjectsCursor,
    ProjectsPage,
    ProjectsStorageBandwidthDaily,
    ProjectInvitationResponse,
} from '@/types/projects';
import { HttpClient } from '@/utils/httpClient';
import { Time } from '@/utils/time';
import { APIError } from '@/utils/error';

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
        const response = await this.http.get(this.ROOT_PATH);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get projects',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const projects = await response.json();
        return projects.map((p: Project) => new Project(
            p.id,
            p.name,
            p.description,
            p.createdAt,
            p.ownerId,
            false,
            p.memberCount,
        ));
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
        const data = {
            name: projectFields.name,
            description: projectFields.description,
            storageLimit: projectLimits.storageLimit.toString(),
            bandwidthLimit: projectLimits.bandwidthLimit.toString(),
        };

        const path = `${this.ROOT_PATH}/${projectId}`;
        const response = await this.http.patch(path, JSON.stringify(data));
        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error || 'Can not update project',
            requestID: response.headers.get('x-request-id'),
        });
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
     * @throws Error
     */
    public async getLimits(projectId: string): Promise<ProjectLimits> {
        const path = `${this.ROOT_PATH}/${projectId}/usage-limits`;
        const response = await this.http.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get usage limits',
                requestID: response.headers.get('x-request-id'),
            });
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
     * @throws Error
     */
    public async getTotalLimits(): Promise<ProjectLimits> {
        const path = `${this.ROOT_PATH}/usage-limits`;
        const response = await this.http.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get total usage limits',
                requestID: response.headers.get('x-request-id'),
            });
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
     * @throws Error
     */
    public async getDailyUsage(projectId: string, start: Date, end: Date): Promise<ProjectsStorageBandwidthDaily> {
        const since = Time.toUnixTimestamp(start).toString();
        const before = Time.toUnixTimestamp(end).toString();
        const path = `${this.ROOT_PATH}/${projectId}/daily-usage?from=${since}&to=${before}`;
        const response = await this.http.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get project daily usage',
                requestID: response.headers.get('x-request-id'),
            });
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

        throw new APIError({
            status: response.status,
            message: 'Can not get project salt',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Fetch owned projects.
     *
     * @returns ProjectsPage
     * @throws Error
     */
    public async getOwnedProjects(cursor: ProjectsCursor): Promise<ProjectsPage> {
        const response = await this.http.get(`${this.ROOT_PATH}/paged?limit=${cursor.limit}&page=${cursor.page}`);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get projects',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const page = await response.json();

        const projects: Project[] = page.projects.map((p: Project) =>
            new Project(
                p.id,
                p.name,
                p.description,
                p.createdAt,
                p.ownerId,
                false,
                p.memberCount,
            ));

        return new ProjectsPage(projects, page.limit, page.offset, page.pageCount, page.currentPage, page.totalCount);
    }

    /**
     * Returns a user's pending project member invitations.
     *
     * @throws Error
     */
    public async getUserInvitations(): Promise<ProjectInvitation[]> {
        const path = `${this.ROOT_PATH}/invitations`;
        const response = await this.http.get(path);
        const result = await response.json();

        if (response.ok) {
            return result.map(jsonInvite => new ProjectInvitation(
                jsonInvite.projectID,
                jsonInvite.projectName,
                jsonInvite.projectDescription,
                jsonInvite.inviterEmail,
                new Date(jsonInvite.createdAt),
            ));
        }

        throw new APIError({
            status: response.status,
            message: result.error || 'Failed to get project invitations',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Handles accepting or declining a user's project member invitation.
     *
     * @throws Error
     */
    public async respondToInvitation(projectID: string, response: ProjectInvitationResponse): Promise<void> {
        const path = `${this.ROOT_PATH}/invitations/${projectID}/respond`;
        const body = { projectID, response };
        const httpResponse = await this.http.post(path, JSON.stringify(body));

        if (httpResponse.ok) return;

        const result = await httpResponse.json();
        throw new APIError({
            status: httpResponse.status,
            message: result.error || 'Failed to respond to project invitation',
            requestID: httpResponse.headers.get('x-request-id'),
        });
    }
}
