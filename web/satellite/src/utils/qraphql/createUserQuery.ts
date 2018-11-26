// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apolloManager from '../apolloManager';
import gql from "graphql-tag";

// Performs graqhQL request.
// Throws an exception if error occurs
export async function createUser(user: User, password: string): Promise<any> {

    let response = apolloManager.mutate(
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
        console.log("cannot create user")
        // TODO: Change with popup
        return null;
    }

    return response;
}

