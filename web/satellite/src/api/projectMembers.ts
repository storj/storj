// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ProjectMember, ProjectMemberCursor, ProjectMembersApi, ProjectMembersPage } from '@/types/projectMembers';

export class ProjectMembersApiGql extends BaseGql implements ProjectMembersApi {

    /**
     * Used for adding team members to project.
     *
     * @param projectId
     * @param emails
     */
    public async add(projectId: string, emails: string[]): Promise<void> {
        const query =
            `mutation($projectId: String!, $emails:[String!]!) {
                addProjectMembers(
                    projectID: $projectId,
                    email: $emails
                ) {id}
            }`;

        const variables = {
            projectId,
            emails,
        };

        await this.mutate(query, variables);
    }

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
                    projectID: $projectId,
                    email: $emails
                ) {id}
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
                    id: $projectId,
                ) {
                    members (
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

        return this.getProjectMembersList(response.data.project.members);
    }

    /**
     * Method for mapping project members page from json to ProjectMembersPage type.
     *
     * @param projectMembers anonymous object from json
     */
    private getProjectMembersList(projectMembers: any): ProjectMembersPage {
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
