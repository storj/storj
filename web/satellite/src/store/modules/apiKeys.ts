// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { API_KEYS_MUTATIONS } from '../mutationConstants';
import { ApiKey, ApiKeysApi } from '@/types/apiKeys';
import { StoreModule } from '@/store';

const {
    FETCH,
    ADD,
    DELETE,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
} = API_KEYS_MUTATIONS;

class ApiKeysState {
    public apiKeys: ApiKey[] = [];
}

/**
 * creates apiKeys module with all dependencies
 *
 * @param api - apiKeys api
 */
export function makeApiKeysModule(api: ApiKeysApi): StoreModule<ApiKeysState> {
    return {
        state: new ApiKeysState(),

        mutations: {
            setAPIKeys(state: any, apiKeys: ApiKey[]) {
                state.apiKeys = apiKeys;
            },
            addAPIKey(state: any, apiKey: ApiKey) {
                state.apiKeys.push(apiKey);
            },
            deleteAPIKey(state: any, ids: string[]) {
                const keysCount = ids.length;

                for (let j = 0; j < keysCount; j++) {
                    state.apiKeys = state.apiKeys.filter((element: ApiKey) => {
                        return element.id !== ids[j];
                    });
                }
            },
            toggleSelection(state: any, apiKeyID: string) {
                state.apiKeys = state.apiKeys.map((apiKey: ApiKey) => {
                    if (apiKey.id === apiKeyID) {
                        apiKey.isSelected = !apiKey.isSelected;
                    }

                    return apiKey;
                });
            },
            clearSelection(state: any) {
                state.apiKeys = state.apiKeys.map((apiKey: ApiKey) => {
                    apiKey.isSelected = false;

                    return apiKey;
                });
            },
        },
        actions: {
            setAPIKeys: async function ({commit, rootGetters}): Promise<ApiKey[]> {
                const projectId = rootGetters.selectedProject.id;

                let apiKeys = await api.get(projectId);

                commit(FETCH, apiKeys);

                return apiKeys;
            },
            createAPIKey: async function ({commit, rootGetters}: any, name: string): Promise<ApiKey> {
                const projectId = rootGetters.selectedProject.id;

                let apiKey = await api.create(projectId, name);

                commit(ADD, apiKey);

                return apiKey;
            },
            deleteAPIKey: async function({commit}: any, ids: string[]): Promise<null> {
                let result = await api.delete(ids);

                commit(DELETE, ids);

                return result;
            },
            toggleAPIKeySelection: function({commit}, apiKeyID: string): void {
                commit(TOGGLE_SELECTION, apiKeyID);
            },
            clearAPIKeySelection: function({commit}): void {
                commit(CLEAR_SELECTION);
            },
            clearAPIKeys: function ({commit}): void {
                commit(FETCH, []);
            },
        },
        getters: {
            selectedAPIKeys: function (state: any): ApiKey[] {
                let keys: ApiKey[] = state.apiKeys;
                let selectedKeys: ApiKey[] = [];

                for (let i = 0; i < keys.length; i++ ) {
                    if (keys[i].isSelected) {
                        selectedKeys.push(keys[i]);
                    }
                }

                return selectedKeys;
            },
            apiKeys: function (state: any): ApiKey[] {
                return state.apiKeys;
            }
        },
    };
}
