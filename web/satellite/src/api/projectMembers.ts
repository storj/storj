// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';

// Performs graqhQL request.
// Throws an exception if error occurs
export async function addProjectMember(userID: string, projectID: string): Promise<any> {
	let response: any = null;
	try {
		response = await apollo.mutate(
			{
				mutation: gql(`
                mutation {
                    addProjectMember(
                        projectID: "${projectID}",
                        userID: "${userID}"
                    ) {id}
                }`
				),
				fetchPolicy: 'no-cache',
			}
		);
	} catch (e) {
		// TODO: replace with popup in future
		console.error(e);
	}

	return response;
}

// Performs graqhQL request.
// Throws an exception if error occurs
export async function deleteProjectMember(userID: string, projectID: string): Promise<any> {
	let response: any = null;
	try {
		response = await apollo.mutate(
			{
				mutation: gql(`
                mutation {
                    deleteProjectMember(
                        projectID: "${projectID}",
                        userID: "${userID}"
                    ) {id}
                }`
				),
				fetchPolicy: 'no-cache',
			}
		);
	} catch (e) {
		// TODO: replace with popup in future
		console.error(e);
	}

	return response;
}

// Performs graqhQL request.
// Throws an exception if error occurs
export async function fetchProjectMembers(projectID: string, limit: string, offset: string): Promise<any> {
	let response: any = null;
	try {
		response = await apollo.query(
			{
				query: gql(`
                query {
                    project(
                        id: "${projectID}",
                    ) {
                        members(limit: ${limit}, offset: ${offset}) {
                            user {
                                id,
                                firstName,
                                lastName,
                                email,
                            },
                            joinedAt
                        }
                    }
                }`
				),
				fetchPolicy: 'no-cache',
			}
		);
	} catch (e) {
		// TODO: replace with popup in future
		console.error(e);
	}

	return response;
}
