// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from "graphql-tag";

// Performs graqhQL request.
// Throws an exception if error occurs
export async function createProject(project: Project): Promise<any> {
    console.log("in api", project);

    let response = await apollo.mutate(
        {
            mutation: gql(`
                mutation {
                    createProject(
                        input: {
                            name: "${project.name}",
                            description: "${project.description}",
                            companyName: "${project.companyName}",
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
                        createdAt            
                        ownerName           
                        companyName           
                        isTermsAccepted
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

// Performs graqhQL request for updating selected project description
export async function updateProject(projectID: string, description: string): Promise<any> {
    let response = await apollo.mutate(
        {
            mutation: gql(`
                mutation {
                    updateProjectDescription(
                        id: "${projectID}",
                        description: "${description}"
                    )
                }`
            ),
            fetchPolicy: "no-cache",
        }
    );

    if(!response){
        // TODO: replace with popup in future
        console.log("cannot update project");

        return null;
    }

    return response;
}

// Performs graqhQL request for deleting selected project
export async function deleteProject(projectID: string): Promise<any> {
    let response = await apollo.mutate(
        {
            mutation: gql(`
                mutation {
                    deleteProject(
                        id: "${projectID}"
                    )
                }`
            ),
            fetchPolicy: "no-cache",
        }
    );

    if(!response){
        // TODO: replace with popup in future
        console.log("cannot delete project");
        return null;
    }

    return response;
}
