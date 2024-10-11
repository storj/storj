// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    DataStamp,
    LimitRequestInfo,
    Project,
    ProjectFields,
    ProjectInvitation,
    ProjectLimits,
    ProjectsApi,
    ProjectsCursor,
    ProjectsPage,
    ProjectDeletionData,
    ProjectsStorageBandwidthDaily,
    ProjectInvitationResponse,
    Emission,
    ProjectConfig,
    UpdateProjectFields,
    UpdateProjectLimitsFields,
} from '@/types/projects';
import { HttpClient } from '@/utils/httpClient';
import { Time } from '@/utils/time';
import { APIError } from '@/utils/error';
import { getVersioning } from '@/types/versioning';
import { DeleteProjectStep } from '@/types/accountActions';

export class ProjectsHttpApi implements ProjectsApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/projects';

    /**
     * Creates project.
     *
     * @param projectFields - contains project information
     * @throws Error
     */
    public async create(projectFields: ProjectFields): Promise<Project> {
        const data = {
            name: projectFields.name,
            description: projectFields.description,
            managePassphrase: projectFields.managePassphrase,
        };

        const response = await this.http.post(this.ROOT_PATH, JSON.stringify(data));
        const result = await response.json();
        if (response.ok) {
            return new Project(
                result.id,
                result.name,
                result.description,
                result.createdAt,
                result.ownerId,
                result.memberCount,
                result.edgeURLOverrides,
                getVersioning(result.versioning),
            );
        }

        throw new APIError({
            status: response.status,
            message: result.error || 'Could not create project',
            requestID: response.headers.get('x-request-id'),
        });
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
        return projects.map(p => new Project(
            p.id,
            p.name,
            p.description,
            p.createdAt,
            p.ownerId,
            p.memberCount,
            p.edgeURLOverrides,
            getVersioning(p.versioning),
            p.storageUsed,
            p.bandwidthUsed,
        ));
    }

    /**
     * Delete project.
     *
     * @throws Error
     */
    public async delete(projectId: string, step: DeleteProjectStep, data: string): Promise<ProjectDeletionData | null> {
        const path = `${this.ROOT_PATH}/${projectId}`;

        const body = JSON.stringify({
            step,
            data,
        });

        const response = await this.http.delete(path, body);

        if (response.ok) {
            return null;
        }

        const result = await response.json();

        if (response.status === 409) {
            return new ProjectDeletionData(
                result.buckets,
                result.apiKeys,
                result.currentUsage,
                result.invoicingIncomplete,
            );
        }

        throw new APIError({
            status: response.status,
            message: result.error || 'Can not delete project. Please try again later',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Fetch config for project.
     *
     * @param projectId - the project's ID
     * @returns ProjectConfig
     * @throws Error
     */
    public async getConfig(projectId: string): Promise<ProjectConfig> {
        const response = await this.http.get(`${this.ROOT_PATH}/${projectId}/config`);
        const result = await response.json();
        if (response.ok) {
            return new ProjectConfig(
                result.versioningUIEnabled,
                result.promptForVersioningBeta,
                result.hasManagedPassphrase,
                result.passphrase ?? '',
                result.isOwnerPaidTier,
                result.role,
                result.objectLockUIEnabled,
            );
        }

        throw new APIError({
            status: response.status,
            message: result.error || 'Could not get project config',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Opt in or out of versioning beta.
     *
     * @param projectId - the project's ID
     * @param status - the new opt-in status
     * @throws Error
     */
    public async setVersioningOptInStatus(projectId: string, status: 'in' | 'out'): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectId}/versioning-opt-${status}`;
        const response = await this.http.patch(path, null);
        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error || `Can not change opt in status`,
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Update project name and description.
     *
     * @param projectId - project ID
     * @param projectFields - project fields
     * @throws Error
     */
    public async update(projectId: string, projectFields: UpdateProjectFields): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectId}`;
        const response = await this.http.patch(path, JSON.stringify(projectFields));
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
     * Update project user specified limits.
     *
     * @param projectId - project ID
     * @param fields - project limits to update
     * @throws Error
     */
    public async updateLimits(projectId: string, fields: UpdateProjectLimitsFields): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectId}/limits`;
        const response = await this.http.patch(path, JSON.stringify(fields));
        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error || 'Can not update limits',
            requestID: response.headers.get('x-request-id'),
        });
    }

    /**
     * Get project limits.
     *
     * @param projectId - project ID
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
            limits.userSetBandwidthLimit,
            limits.userSetStorageLimit,
            limits.bandwidthLimit,
            limits.bandwidthUsed,
            limits.storageLimit,
            limits.storageUsed,
            limits.objectCount,
            limits.segmentCount,
            limits.segmentLimit,
            limits.segmentUsed,
            limits.bucketsLimit,
            limits.bucketsUsed,
        );

    }

    /**
     * Request limit increase.
     *
     * @param projectId - project ID
     * @param info - request information
     * @throws Error
     */
    public async requestLimitIncrease(projectId: string, info: LimitRequestInfo): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectId}/limit-increase`;
        const response = await this.http.post(path, JSON.stringify(info));
        if (response.ok) {
            return;
        }

        const result = await response.json();
        throw new APIError({
            status: response.status,
            message: result.error || 'Can not request increase',
            requestID: response.headers.get('x-request-id'),
        });
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
            null,
            null,
            limits.bandwidthLimit,
            limits.bandwidthUsed,
            limits.storageLimit,
            limits.storageUsed,
        );
    }

    /**
     * Get link to download total usage report for all the projects that user owns.
     *
     * @throws Error
     */
    public getTotalUsageReportLink(start: number, end: number, projectID: string): string {
        return `${this.ROOT_PATH}/usage-report?since=${start.toString()}&before=${end.toString()}&projectID=${projectID}`;
    }

    /**
     * Get project daily usage for specific date range.
     *
     * @param projectId - project ID
     * @param start - since date
     * @param end - before date
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

    public async getEmissionImpact(projectID: string): Promise<Emission> {
        const path = `${this.ROOT_PATH}/${projectID}/emission`;
        const response = await this.http.get(path);
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get project emission impact',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const json = await response.json();
        return json ? new Emission(json.storjImpact, json.hyperscalerImpact, json.savedTrees) : new Emission();
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

        const projects: Project[] = page.projects.map(p =>
            new Project(
                p.id,
                p.name,
                p.description,
                p.createdAt,
                p.ownerId,
                p.memberCount,
                p.edgeURLOverrides,
                getVersioning(p.versioning),
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
