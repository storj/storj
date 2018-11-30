// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '../apolloManager';
import gql from "graphql-tag";

// TODO: all graphql queries should be totally refactored
// Performs graqhQL request.
// Throws an exception if error occurs
export async function createProject(project: Project): Promise<any> {
    let response = apollo.mutate(
        {
            mutation: gql(`
                mutation {
                    createProject(
                        input: {
                            name: "${project.name}",
                            description: "${project.description}",
                            isTermsAccepted: ${project.isTermsAccepted},
                        }
                    )
                }`
            ),
            fetchPolicy: "no-cache",
        }
    );

    if(!response){
        // TODO: replace with popup in future
        console.log("cannot create user");

        return null;
    }

    return response;
}

