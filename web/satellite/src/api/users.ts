// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '../utils/apolloManager';
import gql from 'graphql-tag';

// Performs update user info graphQL mutation request.
// Returns User object if succeed, null otherwise
export async function updateAccountRequest(user: User): Promise<RequestResponse<User>> {
    let result: RequestResponse<User> = {
        errorMessage: '',
        isSuccess: false,
        data: {
            firstName: '',
            lastName: '',
            email: ''
        }
    };

    try {
        let response: any = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        updateAccount (
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

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
            result.data = response.data.updateAccount;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs change password graphQL mutation
// Returns base user fields
export async function changePasswordRequest(password: string, newPassword: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    try {
        let response: any = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        changePassword (
                            password: "${password}",
                            newPassword: "${newPassword}"
                        ) {
                           email 
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
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs Create user graqhQL request.
// Throws an exception if error occurs
// Returns object with newly created user
export async function createUserRequest(user: User, password: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    try {
        let response = await apolloManager.mutate(
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

// Performs graqhQL request.
// Returns Token, User objects.
// Throws an exception if error occurs
export async function getTokenRequest(email: string, password: string): Promise<RequestResponse<string>> {
    let result: RequestResponse<string> = {
        errorMessage: '',
        isSuccess: false,
        data: ''
    };

    try {
        let response: any = await apolloManager.query(
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

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
            result.data = response.data.token.token;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs graqhQL request.
// Returns Token, User objects.
// Throws an exception if error occurs
export async function getUserRequest(): Promise<RequestResponse<User>> {
    let result: RequestResponse<User> = {
        errorMessage: '',
        isSuccess: false,
        data: {
            firstName: '',
            lastName: '',
            email: ''
        }
    };

    try {
        let response: any = await apolloManager.query(
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

        if (response.errors) {
            result.errorMessage = response.errors[0].message;
        } else {
            result.isSuccess = true;
            result.data = response.data.user;
        }
    } catch (e) {
        result.errorMessage = e.message;
    }

    return result;
}

// Performs graqhQL request.
// User object.
// Throws an exception if error occurs
export async function deleteAccountRequest(password: string): Promise<RequestResponse<null>> {
    let result: RequestResponse<null> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    try {
        let response = await apolloManager.mutate(
            {
                mutation: gql(`
                    mutation {
                        deleteAccount(password: "${password}") {
                            email
                        }
                    }`
                ),
                fetchPolicy: 'no-cache'
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

// Performs graphQL request.
// Returns Token string.
// Throws an exception if error occurs
export async function activateAccountRequest(token: string): Promise<RequestResponse<string>> {
	let result: RequestResponse<string> = {
		errorMessage: '',
		isSuccess: false,
		data: ''
	};

	try {
		let response = await apolloManager.mutate(
			{
				mutation: gql(`
				    mutation {
				        activateAccount(input: "${token}") {
				            token
				        }
				    }
				`),
				fetchPolicy: 'no-cache'

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
