// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apollo';
import gql from 'graphql-tag';
import { ProjectMember, ProjectMemberCursor, ProjectMembersPage } from '@/types/projectMembers';
import { RequestResponse } from '@/types/response';

// Performs graqhQL request.
export async function addProjectMembersRequest(projectId: string, emails: string[]): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    let response: any = await apollo.mutate(
        {
            mutation: gql(`
                mutation($projectId: String!, $emails:[String!]!) {
                    addProjectMembers(
                        projectID: $projectId,
                        email: $emails
                    ) {id}
                }`,
            ),
            variables: {
                projectId: projectId,
                emails: emails
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

// Performs graqhQL request.
export async function deleteProjectMembersRequest(projectId: string, emails: string[]): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    let response: any = await apollo.mutate(
        {
            mutation: gql(`
                mutation($projectId: String!, $emails:[String!]!) {
                    deleteProjectMembers(
                        projectID: $projectId,
                        email: $emails
                    ) {id}
                }`
            ),
            variables: {
                projectId: projectId,
                emails: emails
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
    }

    return result;
}

// Performs graqhQL request.
export async function fetchProjectMembersRequest(projectId: string, cursor: ProjectMemberCursor): Promise<RequestResponse<ProjectMembersPage>> {
    let result: RequestResponse<ProjectMembersPage> = {
        errorMessage: '',
        isSuccess: false,
        data: new ProjectMembersPage()
    };

    let response: any = await apollo.query(
        {
            query: gql(`
                query($projectId: String!, $limit: Int!, $search: String!, $page: Int!, $order: Int!, $orderDirection: Int!) {
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
                }`
            ),
            variables: {
                projectId: projectId,
                limit: cursor.limit,
                search: cursor.search,
                page: cursor.page,
                order: cursor.order,
                orderDirection: cursor.orderDirection,
            },
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = getProjectMembersList(response.data.project.members);
    }

    return result;
}

function getProjectMembersList(projectMembers: any): ProjectMembersPage {
    if (!projectMembers) {
        return new ProjectMembersPage();
    }

    const projectMembersPage: ProjectMembersPage = new ProjectMembersPage();
    projectMembersPage.projectMembers = projectMembers.projectMembers.map(key => new ProjectMember(key.user.fullName, key.user.shortName, key.user.email, key.joinedAt, key.user.id));

    projectMembersPage.search = projectMembers.search;
    projectMembersPage.limit = projectMembers.limit;
    projectMembersPage.order = projectMembers.order;
    projectMembersPage.orderDirection = projectMembers.orderDirection;
    projectMembersPage.pageCount = projectMembers.pageCount;
    projectMembersPage.currentPage = projectMembers.currentPage;
    projectMembersPage.totalCount = projectMembers.totalCount;

    return projectMembersPage;
}
