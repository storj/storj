// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { ProjectMemberSortByEnum } from '@/utils/constants/ProjectMemberSortEnum';
import { TeamMember } from '@/types/teamMembers';
import { RequestResponse } from '@/types/response';

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
export async function fetchProjectMembersRequest(projectID: string, limit: string, offset: string, sortBy: ProjectMemberSortByEnum, searchQuery: string): Promise<RequestResponse<TeamMember[]>> {
    let result: RequestResponse<TeamMember[]> = {
        errorMessage: '',
        isSuccess: false,
        data: []
    };

    let response: any = await apollo.query(
        {
            query: gql(`
            query {
                project(
                    id: "${projectID}",
                ) {
                    members(limit: ${limit}, offset: ${offset}, order: ${sortBy}, search: "${searchQuery}") {
                        user {
                            id,
                            fullName,
                            shortName,
                            email
                        },
                        joinedAt
                    }
                }
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
        result.data = getProjectMembersList(response.data.project.members);
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

function getProjectMembersList(projectMembers: any[]): TeamMember[] {
    if (!projectMembers) {
        return [];
    }

    return projectMembers.map(key => new TeamMember(key.user.fullName, key.user.shortName, key.user.email, '', key.user.id));
}
