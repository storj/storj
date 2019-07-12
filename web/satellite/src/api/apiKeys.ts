// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import apollo from '@/utils/apolloManager';
import gql from 'graphql-tag';
import { ApiKey } from '@/types/apiKeys';

export async function fetchAPIKeys(projectID: string): Promise<RequestResponse<ApiKey[]>> {
    let result: RequestResponse<ApiKey[]> = {
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
        result.data = getApiKeysList(response.data.project.apiKeys);
    }

    return result;
}

export async function createAPIKey(projectID: string, name: string): Promise<RequestResponse<ApiKey>> {
    let result: RequestResponse<ApiKey> = {
        errorMessage: '',
        isSuccess: false,
        data: new ApiKey('', '', '', ''),
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
        let key: any = response.data.createAPIKey.keyInfo;
        let secret: string = response.data.createAPIKey.key;

        result.data = new ApiKey(key.id, key.name, key.createdAt, secret);
    }

    return result;
}

export async function deleteAPIKeys(ids: string[]): Promise<RequestResponse<null>> {
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

function getApiKeysList(apiKeys: Array<any>): ApiKey[] {
    if (!apiKeys) {
        return [];
    }

    return apiKeys.map(key => new ApiKey(key.id, key.name, key.createdAt, ''));
}
