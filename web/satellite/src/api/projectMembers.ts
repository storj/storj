// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ProjectMember, ProjectMemberCursor, ProjectMembersApi, ProjectMembersPage } from '@/types/projectMembers';

export class ProjectMembersApiGql extends BaseGql implements ProjectMembersApi {
    public async add(projectId: string, emails: string[]): Promise<null> {
        const query =
            `mutation($projectId: String!, $emails:[String!]!) {
                addProjectMembers(
                    projectID: $projectId,
                    email: $emails
                ) {id}
            }`;

        const variables = {
            projectId: projectId,
            emails: emails
        };

        const response = await this.mutate(query, variables);

        return response.data.addProjectMembers;
    }

    public async delete(projectId: string, emails: string[]): Promise<null> {
        const query =
            `mutation($projectId: String!, $emails:[String!]!) {
                deleteProjectMembers(
                    projectID: $projectId,
                    email: $emails
                ) {id}
            }`;

        const variables = {
            projectId: projectId,
            emails: emails
        };

        const response = await this.mutate(query, variables);

        return response.data.deleteProjectMembers;
    }

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

    private  getProjectMembersList(projectMembers: any): ProjectMembersPage {
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

}

// export async function fetchProjectMembersRequest(projectId: string, cursor: ProjectMemberCursor): Promise<RequestResponse<ProjectMembersPage>> {
//     let result: RequestResponse<ProjectMembersPage> = {
//         errorMessage: '',
//         isSuccess: false,
//         data: new ProjectMembersPage()
//     };
//
//     let response: any = await apollo.query(
//         {
//             query: gql(`
//                 query($projectId: String!, $limit: Int!, $search: String!, $page: Int!, $order: Int!, $orderDirection: Int!) {
//                     project (
//                         id: $projectId,
//                     ) {
//                         members (
//                             cursor: {
//                                 limit: $limit,
//                                 search: $search,
//                                 page: $page,
//                                 order: $order,
//                                 orderDirection: $orderDirection
//                             }
//                         ) {
//                             projectMembers {
//                                 user {
//                                     id,
//                                     fullName,
//                                     shortName,
//                                     email
//                                 },
//                                 joinedAt
//                             },
//                             search,
//                             limit,
//                             order,
//                             pageCount,
//                             currentPage,
//                             totalCount
//                         }
//                     }
//                 }`
//             ),
//             variables: {
//                 projectId: projectId,
//                 limit: cursor.limit,
//                 search: cursor.search,
//                 page: cursor.page,
//                 order: cursor.order,
//                 orderDirection: cursor.orderDirection,
//             },
//             fetchPolicy: 'no-cache',
//             errorPolicy: 'all',
//         }
//     );
//
//     if (response.errors) {
//         result.errorMessage = response.errors[0].message;
//     } else {
//         result.isSuccess = true;
//         result.data = getProjectMembersList(response.data.project.members);
//     }
//
//     return result;
// }
//
//
// private function addProjectMembersRequest(projectId: string, emails: string[]): Promise<RequestResponse<null>> {
//     let result: RequestResponse<null> = {
//         errorMessage: '',
//         isSuccess: false,
//         data: null
//     };
//
//     let response: any = await apollo.mutate(
//         {
//             mutation: gql(`
//                 `,
//             ),
//             variables: {
//                 projectId: projectId,
//                 emails: emails
//             },
//             fetchPolicy: 'no-cache',
//             errorPolicy: 'all',
//         }
//     );
//
//     if (response.errors) {
//         result.errorMessage = response.errors[0].message;
//     } else {
//         result.isSuccess = true;
//     }
//
//     return result;
// }
//
// export async function deleteProjectMembersRequest(projectId: string, emails: string[]): Promise<RequestResponse<null>> {
//     let result: RequestResponse<null> = {
//         errorMessage: '',
//         isSuccess: false,
//         data: null
//     };
//
//     let response: any = await apollo.mutate(
//         {
//             mutation: gql(`
//                 mutation($projectId: String!, $emails:[String!]!) {
//                     deleteProjectMembers(
//                         projectID: $projectId,
//                         email: $emails
//                     ) {id}
//                 }`
//             ),
//             variables: {
//                 projectId: projectId,
//                 emails: emails
//             },
//             fetchPolicy: 'no-cache',
//             errorPolicy: 'all',
//         }
//     );
//
//     if (response.errors) {
//         result.errorMessage = response.errors[0].message;
//     } else {
//         result.isSuccess = true;
//     }
//
//     return result;
// }
