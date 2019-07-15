// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { API_KEYS_MUTATIONS } from '../mutationConstants';
import { createAPIKey, deleteAPIKeys, fetchAPIKeys } from '@/api/apiKeys';
import { API_KEYS_ACTIONS } from '@/utils/constants/actionNames';
import { ApiKey } from '@/types/apiKeys';
import { RequestResponse } from '@/types/response';

export const apiKeysModule = {
    state: {
        apiKeys: [],
    },
    mutations: {
        [API_KEYS_MUTATIONS.FETCH](state: any, apiKeys: ApiKey[]) {
            state.apiKeys = apiKeys;
        },
        [API_KEYS_MUTATIONS.ADD](state: any, apiKey: ApiKey) {
            state.apiKeys.push(apiKey);
        },
        [API_KEYS_MUTATIONS.DELETE](state: any, ids: string[]) {
            const keysCount = ids.length;

            for (let j = 0; j < keysCount; j++) {
                state.apiKeys = state.apiKeys.filter((element: ApiKey) => {
                    return element.id !== ids[j];
                });
            }
        },
        [API_KEYS_MUTATIONS.TOGGLE_SELECTION](state: any, apiKeyID: string) {
            state.apiKeys = state.apiKeys.map((apiKey: ApiKey) => {
                if (apiKey.id === apiKeyID) {
                    apiKey.isSelected = !apiKey.isSelected;
                }

                return apiKey;
            });
        },
        [API_KEYS_MUTATIONS.CLEAR_SELECTION](state: any) {
            state.apiKeys = state.apiKeys.map((apiKey: ApiKey) => {
                apiKey.isSelected = false;

                return apiKey;
            });
        },
    },
    actions: {
        [API_KEYS_ACTIONS.FETCH]: async function ({commit, rootGetters}): Promise<RequestResponse<ApiKey[]>> {
            const projectId = rootGetters.selectedProject.id;

            let fetchResult: RequestResponse<ApiKey[]> = await fetchAPIKeys(projectId);
            if (fetchResult.isSuccess) {
                commit(API_KEYS_MUTATIONS.FETCH, fetchResult.data);
            }

            return fetchResult;
        },
        [API_KEYS_ACTIONS.CREATE]: async function ({commit, rootGetters}: any, name: string): Promise<RequestResponse<ApiKey>> {
            const projectId = rootGetters.selectedProject.id;

            let result: RequestResponse<ApiKey> = await createAPIKey(projectId, name);

            if (result.isSuccess) {
                commit(API_KEYS_MUTATIONS.ADD, result.data);
            }

            return result;
        },
        [API_KEYS_ACTIONS.DELETE]: async function({commit}: any, ids: string[]): Promise<RequestResponse<null>> {
            let result = await deleteAPIKeys(ids);

            if (result.isSuccess) {
                commit(API_KEYS_MUTATIONS.DELETE, ids);
            }

            return result;
        },
        [API_KEYS_ACTIONS.TOGGLE_SELECTION]: function({commit}, apiKeyID: string): void {
            commit(API_KEYS_MUTATIONS.TOGGLE_SELECTION, apiKeyID);
        },
        [API_KEYS_ACTIONS.CLEAR_SELECTION]: function({commit}): void {
            commit(API_KEYS_MUTATIONS.CLEAR_SELECTION);
        },
        [API_KEYS_ACTIONS.CLEAR]: function ({commit}): void {
            commit(API_KEYS_MUTATIONS.FETCH, []);
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
        }
    },
};
