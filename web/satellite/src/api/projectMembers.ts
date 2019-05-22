// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { ProjectMemberSortByEnum } from '@/utils/constants/ProjectMemberSortEnum';
import { ProjectMemberCursor, ProjectMembersPage, TeamMemberModel } from '@/types/projects';

// Performs graqhQL request.
export async function addProjectMembersRequest(projectID: string, emails: string[]): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    let response: any = await apollo.mutate(
        {
            mutation: gql(`
            mutation {
                addProjectMembers(
                    projectID: "${projectID}",
                    email: [${prepareEmailList(emails)}]
                ) {id}
            }`,
            ),
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
export async function deleteProjectMembersRequest(projectID: string, emails: string[]): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    let response: any = await apollo.mutate(
        {
            mutation: gql(`
            mutation {
                deleteProjectMembers(
                    projectID: "${projectID}",
                    email: [${prepareEmailList(emails)}]
                ) {id}
            }`
            ),
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
export async function fetchProjectMembersRequest(projectID: string, cursor: ProjectMemberCursor): Promise<RequestResponse<ProjectMembersPage>> {
    let result: RequestResponse<ProjectMembersPage> = {
        errorMessage: '',
        isSuccess: false,
        data: {} as ProjectMembersPage
    };
    let response: any = await apollo.query(
        {
            query: gql(
                `
                query {
                    project(id: "${projectID}") {
                        members(
                            cursor: {
                                limit: ${cursor.limit}, 
                                search: "${cursor.search}",
                                page: ${cursor.page},
                                order: ${cursor.order}
                            }
                        ) {
                            projectMembers{
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
                            offset, 
                            pageCount, 
                            currentPage,
                            totalCount
                        }
                    }
                }
            `
            ),
            fetchPolicy: 'no-cache',
            errorPolicy: 'all',
        }
    );

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.project.members;
    }

    return result;
}

function prepareEmailList(emails: string[]): string {
    let emailString: string = '';

    emails.forEach(email => {
        emailString += `"${email}", `;
    });

    return emailString;
}
