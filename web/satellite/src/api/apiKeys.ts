// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ApiKey, ApiKeysApi } from '@/types/apiKeys';

export class ApiKeysApiGql extends BaseGql implements ApiKeysApi {
    /**
     * Fetch apiKey
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
     * Used to create account
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
        let key: any = response.data.createAPIKey.keyInfo;
        let secret: string = response.data.createAPIKey.key;

        return new ApiKey(key.id, key.name, key.createdAt, secret);
    }

    /**
     * Used to delete account
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
