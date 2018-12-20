// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '../utils/apolloManager';
import gql from 'graphql-tag';

// Performs update user info graphQL mutation request.
// Returns User object if succeed, null otherwise
export async function updateBasicUserInfoRequest(user: User) {
    let response: any = null;

    try {
        response = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        updateUser (
                            id: "${user.id}",
                            input: {
                                email: "${user.email}",
                                firstName: "${user.firstName}",
                                lastName: "${user.lastName}"
                            }
                        ) {
                            email,
                            firstName,
                            lastName
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

// Performs change password graphQL mutation
// Returns base user fields
export async function updatePasswordRequest(userId: string, password: string) {
    let response: any = null;

    try {
        response = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        updateUser (
                            id: "${userId}",
                            input: {
                                password: "${password}"
                            }
                        ) {
                            email,
                            firstName,
                            lastName
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

// Performs Create user graqhQL request.
// Throws an exception if error occurs
// Returns object with newly created user
export async function createUserRequest(user: User, password: string): Promise<any> {
    let response: any = null;

    try {
        response = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        createUser(
                            input:{
                                email: "${user.email}",
                                password: "${password}",
                                firstName: "${user.firstName}",
                                lastName: "${user.lastName}",
                            }
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

// Performs graqhQL request.
// Returns Token, User objects.
// Throws an exception if error occurs
export async function getTokenRequest(email: string, password: string): Promise<any> {
    let response: any = null;

    try {
        response = await apolloManager.query(
            {
                query: gql(`
                    query {
                        token(email: "${email}",
                              password: "${password}") {
                                  token
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

// Performs graqhQL request.
// Returns Token, User objects.
// Throws an exception if error occurs
export async function getUserRequest(): Promise<any> {
    let response: any = null;

    try {
        response = await apolloManager.query(
            {
                query: gql(`
                    query {
                        user {
                            firstName,
                            lastName,
                            email,
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

// Performs graqhQL request.
// User object.
// Throws an exception if error occurs
export async function deleteUserAccountRequest(password: string): Promise<any> {
    let response: any = null;

    try {
        response = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        deleteUser(password: "${password}") {
                            id
                        }
                    }`
                ),
                fetchPolicy: 'no-cache'
            }
        );
    } catch (e) {
        console.error(e);
    }

	return response;
}
