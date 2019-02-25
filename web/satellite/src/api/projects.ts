// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';

// Performs graqhQL request for project creation.
export async function createProjectRequest(project: Project): Promise<RequestResponse<Project>> {
    let result: RequestResponse<Project> = {
        errorMessage: '',
        isSuccess: false,
        data: project
    };

    try {
        let response: any = await apollo.mutate(
            {
                mutation: gql(`
					mutation {
						createProject(
							input: {
								name: "${project.name}",
								description: "${project.description}",
							}
						) {id}
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
            result.data.id = response.data.createProject.id;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs graqhQL request for fetching all projects of current user.
export async function fetchProjectsRequest(): Promise<RequestResponse<Project[]>> {
    let result: RequestResponse<Project[]> = {
        errorMessage: '',
        isSuccess: false,
        data: []
    };

    try {
        let response: any = await apollo.query(
            {
                query: gql(`
					query {
						myProjects{
							name
							id
							description
							createdAt
						}
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
            result.data = response.data.myProjects;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs graqhQL request for updating selected project description
export async function updateProjectRequest(projectID: string, description: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    try {
        let response: any = await apollo.mutate(
            {
                mutation: gql(`
					mutation {
						updateProjectDescription(
							id: "${projectID}",
							description: "${description}"
						) {name}
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs graqhQL request for deleting selected project
export async function deleteProjectRequest(projectID: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    try {
        let response = await apollo.mutate(
            {
                mutation: gql(`
					mutation {
						deleteProject(
							id: "${projectID}"
						) {name}
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}
