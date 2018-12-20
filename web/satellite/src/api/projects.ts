// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';

// Performs graqhQL request.
// Throws an exception if error occurs
export async function createProject(project: Project): Promise<any> {
    let response: any = null;

    try {
        response = await apollo.mutate(
            {
                mutation: gql(`
					mutation {
						createProject(
							input: {
								name: "${project.name}",
								description: "${project.description}",
								isTermsAccepted: ${project.isTermsAccepted},
							}
						) {id}
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );
    } catch (e) {
		console.error(e);
    }

    return response;
}

// Performs graqhQL request for fetching all projects of current user.
export async function fetchProjects(): Promise<any> {
    let response: any = null;

	try {
        response = await apollo.query(
            {
                query: gql(`
					query {
						myProjects{
							name
							id
							description
							createdAt            
							isTermsAccepted
						}
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );

    } catch (e) {
		console.error(e);
	}

    return response;
}

// Performs graqhQL request for updating selected project description
export async function updateProject(projectID: string, description: string): Promise<any> {
    let response: any = null;

    try {
        response = await apollo.mutate(
            {
                mutation: gql(`
					mutation {
						updateProjectDescription(
							id: "${projectID}",
							description: "${description}"
						)
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );
    } catch (e) {
		console.error(e);
    }

    return response;
}

// Performs graqhQL request for deleting selected project
export async function deleteProject(projectID: string): Promise<any> {
    let response: any = null;

    try {
        response = await apollo.mutate(
            {
                mutation: gql(`
					mutation {
						deleteProject(
							id: "${projectID}"
						)
					}`
                ),
                fetchPolicy: 'no-cache',
            }
        );
    } catch (e) {
		console.error(e);
    }

    return response;
}
