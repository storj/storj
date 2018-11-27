// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '../apolloManager';
import gql from "graphql-tag";

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

    console.log(response);
    if (!response) {
        console.error("No token received");
        // TODO: Change with popup
        return null;
    }

    return response;
}