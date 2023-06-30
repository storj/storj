// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ProjectInvitationItemModel, ProjectMember, ProjectMemberCursor, ProjectMembersApi, ProjectMembersPage } from '@/types/projectMembers';
import { HttpClient } from '@/utils/httpClient';

export class ProjectMembersApiGql extends BaseGql implements ProjectMembersApi {
    private readonly http: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/projects';

    /**
     * Used for deleting team members from project.
     *
     * @param projectId
     * @param emails
     */
    public async delete(projectId: string, emails: string[]): Promise<void> {
        const query =
            `mutation($projectId: String!, $emails:[String!]!) {
                deleteProjectMembers(
                    publicId: $projectId,
                    email: $emails
                ) {publicId}
            }`;

        const variables = {
            projectId,
            emails,
        };

        await this.mutate(query, variables);
    }

    /**
     * Used for fetching team members related to project.
     *
     * @param projectId
     * @param cursor for pagination
     */
    public async get(projectId: string, cursor: ProjectMemberCursor): Promise<ProjectMembersPage> {
        const query =
            `query($projectId: String!, $limit: Int!, $search: String!, $page: Int!, $order: Int!, $orderDirection: Int!) {
                project (
                    publicId: $projectId,
                ) {
                    membersAndInvitations (
                        cursor: {
                            limit: $limit,
                            search: $search,
                            page: $page,
                            order: $order,
                            orderDirection: $orderDirection
                        }
                    ) {
                        projectMembers {
                            user {
                                id,
                                fullName,
                                shortName,
                                email
                            },
                            joinedAt
                        },
                        projectInvitations {
                            email,
                            createdAt,
                            expired
                        },
                        search,
                        limit,
                        order,
                        pageCount,
                        currentPage,
                        totalCount
                    }
                }
            }`;

        const variables = {
            projectId: projectId,
            limit: cursor.limit,
            search: cursor.search,
            page: cursor.page,
            order: cursor.order,
            orderDirection: cursor.orderDirection,
        };

        const response = await this.query(query, variables);

        return this.getProjectMembersList(response.data.project.membersAndInvitations);
    }

    /**
     * Handles inviting users to a project.
     *
     * @throws Error
     */
    public async invite(projectID: string, emails: string[]): Promise<void> {
        const path = `${this.ROOT_PATH}/${projectID}/invite`;
        const body = { emails };
        const httpResponse = await this.http.post(path, JSON.stringify(body));

        if (httpResponse.ok) return;

        const result = await httpResponse.json();
        throw new Error(result.error || 'Failed to send project invitations');
    }

    /**
     * Get invite link for the specified project and email.
     *
     * @throws Error
     */
    public async getInviteLink(projectID: string, email: string): Promise<string> {
        const path = `${this.ROOT_PATH}/${projectID}/invite-link?email=${encodeURIComponent(email)}`;
        const httpResponse = await this.http.get(path);

        if (httpResponse.ok) {
            return await httpResponse.json();
        }

        throw new Error('Can not get invite link');
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
            key.user.fullName,
            key.user.shortName,
            key.user.email,
            new Date(key.joinedAt),
            key.user.id,
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
