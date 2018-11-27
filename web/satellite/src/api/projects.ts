// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from "graphql-tag";

// Performs graqhQL request.
// Throws an exception if error occurs
export async function createProject(project: Project): Promise<any> {
    let response = await apollo.mutate(
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
        console.log("cannot create project");

        return null;
    }

    return response;
}

// Performs graqhQL request for fetching all projects of current user.
export async function fetchProjects(): Promise<any> {
    let response = await apollo.query(
        {
            query: gql(`
                query {
                    myProjects{
                        name
                        id
                        description                       
                    }
                }`
            ),
            fetchPolicy: "no-cache",
        }
    );

    if(!response){
        // TODO: replace with popup in future
        console.log("cannot fetch projects");

        return null;
    }

    return response;
}
