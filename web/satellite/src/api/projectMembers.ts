// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    ProjectInvitationItemModel,
    ProjectMember,
    ProjectMemberCursor,
    ProjectMembersApi,
    ProjectMembersPage,
    ProjectRole,
} from '@/types/projectMembers';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';

export class ProjectMembersHttpApi implements ProjectMembersApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/projects';

    /**
     * Used for deleting team members from project.
     *
     * @param projectId
     * @param emails
     * @param csrfProtectionToken
     */
    public async delete(projectId: string, emails: string[], csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectId}/members?emails=${encodeURIComponent(emails.toString())}`;
        const response = await this.http.delete(path, null, { csrfProtectionToken });
        if (!response.ok) {
            const result = await response.json();
            throw new APIError({
                status: response.status,
                message: result.error || 'Failed to delete project members and invitations',
                requestID: response.headers.get('x-request-id'),
            });
        }

    }

    /**
     * Used for fetching team members and invitations related to project.
     *
     * @param projectId
     * @param cursor for pagination
     */
    public async get(projectId: string, cursor: ProjectMemberCursor): Promise<ProjectMembersPage> {
        const path = `${this.ROOT_PATH}/${projectId}/members?limit=${cursor.limit}&page=${cursor.page}&order=${cursor.order}&order-direction=${cursor.orderDirection}&search=${cursor.search}`;
        const response = await this.http.get(path);
        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || 'Failed to get project members and invitations',
                requestID: response.headers.get('x-request-id'),
            });
        }
        return this.getProjectMembersList(result);
    }

    /**
     * Handles inviting a user to a project.
     *
     * @throws Error
     */
    public async invite(projectID: string, email: string, csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectID}/invite/${encodeURIComponent(email)}`;
        const httpResponse = await this.http.post(path, null, { csrfProtectionToken });

        if (httpResponse.ok) return;

        const result = await httpResponse.json();
        throw new APIError({
            status: httpResponse.status,
            message: result.error || 'Failed to send project invitations',
            requestID: httpResponse.headers.get('x-request-id'),
        });
    }

    /**
     * Used for fetching team member related to project.
     *
     * @throws Error
     */
    public async getSingleMember(projectID: string, memberID: string): Promise<ProjectMember> {
        const path = `${this.ROOT_PATH}/${projectID}/members/${memberID}`;
        const response = await this.http.get(path);

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || `Failed to get project member`,
                requestID: response.headers.get('x-request-id'),
            });
        }

        return new ProjectMember(
            '',
            '',
            '',
            new Date(result.joinedAt),
            result.id,
            result.role,
        );
    }

    /**
     * Handles updating project member's role.
     *
     * @throws Error
     */
    public async updateRole(projectID: string, memberID: string, role: ProjectRole, csrfProtectionToken: string): Promise<ProjectMember> {
        const path = `${this.ROOT_PATH}/${projectID}/members/${memberID}`;
        const body = role === ProjectRole.Admin ? 0 : 1;
        const response = await this.http.patch(path, body.toString(), { csrfProtectionToken });

        const result = await response.json();
        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: result.error || `Failed update member's role`,
                requestID: response.headers.get('x-request-id'),
            });
        }

        return new ProjectMember(
            result.fullName,
            result.shortName,
            result.email,
            new Date(result.joinedAt),
            result.id,
            result.role,
        );
    }

    /**
     * Handles resending invitations to project.
     *
     * @throws Error
     */
    public async reinvite(projectID: string, emails: string[], csrfProtectionToken: string): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectID}/reinvite`;
        const body = { emails };
        const httpResponse = await this.http.post(path, JSON.stringify(body), { csrfProtectionToken });

        if (httpResponse.ok) return;

        const result = await httpResponse.json();
        throw new APIError({
            status: httpResponse.status,
            message: result.error || 'Failed to resend project invitations',
            requestID: httpResponse.headers.get('x-request-id'),
        });
    }

    /**
     * Get invite link for the specified project and email.
     *
     * @throws Error
     */
    public async getInviteLink(projectID: string, email: string): Promise<string> {
        const path = `${this.ROOT_PATH}/${projectID}/invite-link?email=${encodeURIComponent(email)}`;
        const httpResponse = await this.http.get(path);
        const result = await httpResponse.json();
        if (httpResponse.ok) {
            return result;
        }

        throw new APIError({
            status: httpResponse.status,
            message: result.error || 'Can not get invite link',
            requestID: httpResponse.headers.get('x-request-id'),
        });
    }

    /**
     * Method for mapping project members page from json to ProjectMembersPage type.
     *
     * @param projectMembers anonymous object from json
     */
    private getProjectMembersList(projectMembers: any): ProjectMembersPage { // eslint-disable-line @typescript-eslint/no-explicit-any
        if (!projectMembers) {
            return new ProjectMembersPage();
        }

        const projectMembersPage: ProjectMembersPage = new ProjectMembersPage();
        projectMembersPage.projectMembers = projectMembers.projectMembers.map(key => new ProjectMember(
            key.fullName,
            key.shortName,
            key.email,
            new Date(key.joinedAt),
            key.id,
            key.role,
        ));
        projectMembersPage.projectInvitations = projectMembers.projectInvitations.map(key => new ProjectInvitationItemModel(
            key.email,
            new Date(key.createdAt),
            key.expired,
        ));

        projectMembersPage.search = projectMembers.search;
        projectMembersPage.limit = projectMembers.limit;
        projectMembersPage.order = projectMembers.order;
        projectMembersPage.orderDirection = projectMembers.orderDirection;
        projectMembersPage.pageCount = projectMembers.pageCount;
        projectMembersPage.currentPage = projectMembers.currentPage;
        projectMembersPage.totalCount = projectMembers.totalCount;

        return projectMembersPage;
    }
}
