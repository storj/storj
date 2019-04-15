// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';

export async function fetchAPIKeys(projectID: string) {
    let result: RequestResponse<any[]> = {
        errorMessage: '',
        isSuccess: false,
        data: []
    };

    let response: any = await apollo.query({
        query: gql(
            `query {
                project(
                    id: "${projectID}",
                ) {
                    apiKeys {
                        id,
                        name,
                        createdAt
                    }
                }
            }`
        ),
        fetchPolicy: 'no-cache',
        errorPolicy: 'all',
    });

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.project.apiKeys;
    }

    return result;
}

export async function createAPIKey(projectID: string, name: string) {
    let result: RequestResponse<any> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    let response: any = await apollo.mutate({
        mutation: gql(
            `mutation {
                createAPIKey(
                    projectID: "${projectID}",
                    name: "${name}"
                ) {
                    key,
                    keyInfo {
                        id,
                        name,
                        createdAt
                    }
                }
            }`
        ),
        fetchPolicy: 'no-cache',
        errorPolicy: 'all',
    });

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = {
            key: response.data.createAPIKey.key,
            keyInfo: response.data.createAPIKey.keyInfo
        };
    }

    return result;
}

export async function deleteAPIKeys(ids: string[]) {
    let result: RequestResponse<any> = {
        errorMessage: '',
        isSuccess: false,
        data: null
    };

    let response: any = await apollo.mutate({
        mutation: gql(
            `mutation {
                deleteAPIKeys(id: [${prepareIdList(ids)}]) {
                    id
                }
            }`
        ),
        fetchPolicy: 'no-cache',
        errorPolicy: 'all',
    });

    if (response.errors) {
        result.errorMessage = response.errors[0].message;
    } else {
        result.isSuccess = true;
        result.data = response.data.deleteAPIKeys;
    }

    return result;
}

function prepareIdList(ids: string[]): string {
    let idString: string = '';

    ids.forEach(id => {
        idString += `"${id}", `;
    });

    return idString;
}
