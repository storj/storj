// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ApiKey, ApiKeysApi } from '@/types/apiKeys';

/**
 * ApiKeysApiGql is a graphql implementation of ApiKeys API.
 * Exposes all apiKey-related functionality
 */
export class ApiKeysApiGql extends BaseGql implements ApiKeysApi {
    /**
     * Fetch apiKeys
     *
     * @returns ApiKey
     * @throws Error
     */
    public async get(projectId: string): Promise<ApiKey[]> {
        const query =
            ` query($projectId: String!) {
                project(
                    id: $projectId,
                ) {
                    apiKeys {
                        id,
                        name,
                        createdAt
                    }
                }
            }`;

        const variables = {
            projectId
        };

        const response = await this.query(query, variables);

        return this.getApiKeysList(response.data.project.apiKeys);
    }

    /**
     * Used to create apiKey
     *
     * @param projectId - stores current project id
     * @param name - name of apiKey that will be created
     * @returns ApiKey
     * @throws Error
     */
    public async create(projectId: string, name: string): Promise<ApiKey> {
        const query =
            `mutation($projectId: String!, $name: String!) {
                createAPIKey(
                    projectID: $projectId,
                    name: $name
                ) {
                    key,
                    keyInfo {
                        id,
                        name,
                        createdAt
                    }
                }
            }`;

        const variables = {
            projectId,
            name,
        };

        const response = await this.mutate(query, variables);
        const key: any = response.data.createAPIKey.keyInfo;
        const secret: string = response.data.createAPIKey.key;

        return new ApiKey(key.id, key.name, key.createdAt, secret);
    }

    /**
     * Used to delete apiKey
     *
     * @param ids - ids of apiKeys that will be deleted
     * @throws Error
     */
    public async delete(ids: string[]): Promise<null> {
        const query =
            `mutation($id: [String!]!) {
                deleteAPIKeys(id: $id) {
                    id
                }
            }`;

        const variables = {
            id: ids,
        };

        const response = await this.mutate(query, variables);

        return response.data.deleteAPIKeys;
    }

    private getApiKeysList(apiKeys: ApiKey[]): ApiKey[] {
        if (!apiKeys) {
            return [];
        }

        return apiKeys.map(key => new ApiKey(key.id, key.name, key.createdAt, ''));
    }
}
