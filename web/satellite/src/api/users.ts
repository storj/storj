// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from "../utils/apolloManager";
import gql from "graphql-tag";

// Performs update user info graphQL mutation request.
// Returns User object if succeed, null otherwise
export async function updateBasicUserInfo(user: User) {
    let response = await apolloManager.mutate(
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
                }
            `),
            fetchPolicy: "no-cache",
        }
    );

    if (!response) {
        return null;
    }

    return response;
}

// Performs update company info graphQL mutation request.
// Returns Company object if succeed, null otherwise
export async function updateCompanyInfo(userId: string, company: Company) {
    let response = await apolloManager.mutate(
        {
            mutation:gql(`
                mutation {
	                updateCompany(
                        userID:"${userId}",
                        input:{
                            name:"${company.name}",
                            address:"${company.address}",
                            country:"${company.country}",
                            city:"${company.city}",
                            state:"${company.state}",
                            postalCode:"${company.postalCode}"
                        }
                    ){
                        name,
                        address,
                        country,
                        city,
                        state,
                        postalCode
                    }
                }
           `)
        }
    );

    if (!response) {
        return null;
    }

    return response;
}

// Performs change password graphQL mutation
// Returns base user fields
export async function updatePassword(userId: string, password: string) {
    let response = await apolloManager.mutate(
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
                }
            `),
            fetchPolicy: "no-cache",
        }
    );

    if (!response) {
        return null;
    }

    return response;
}

// Performs Create user graqhQL request.
// Throws an exception if error occurs
// Returns object with newly created user
export async function createUser(user: User, password: string): Promise<any> {
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
                            company: {
                                name: "${user.company.name}",
                                address: "${user.company.address}",
                                country: "${user.company.country}",
                                city: "${user.company.city}",
                                state: "${user.company.state}",
                                postalCode: "${user.company.postalCode}"
                            }
                        }
                    )
                }
           `),
            fetchPolicy: "no-cache",
        }
    );

    if (!response) {
        return null;
    }

    return response;
}


// Performs graqhQL request.
// Returns Token, User objects.
// Throws an exception if error occurs
export async function login(email: string, password: string): Promise<any> {
    let response = await apolloManager.query(
        {
            query: gql(`
                query {
                    token(email: "${email}",
                        password: "${password}") {
                            token,
                            user{
                                id,
                                firstName,
                                lastName,
                                email,
                                company{
                                    name,
                                    address,
                                    country,
                                    city,
                                    state,
                                    postalCode
                                }
                            }
                    }
                }`),
            fetchPolicy: "no-cache",
        }
    );

    if (!response) {
        return null;
    }

    return response;
}

// Performs graqhQL request.
// User object.
// Throws an exception if error occurs
export async function deleteUserAccount(userId: string): Promise<any> {
    let response = await apolloManager.mutate(
        {
            mutation: gql(`
                mutation {
                    deleteUser(id: "${userId}") {
                        id
                    }
                }
            `),
            fetchPolicy: "no-cache"
        }
    );

    if(!response) {
        return null;
    }

    return response;
}
