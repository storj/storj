// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { API_KEYS_MUTATIONS } from '../mutationConstants';
import { createAPIKey, deleteAPIKey, fetchAPIKeys } from "@/api/apiKeys";
import { API_KEYS_ACTIONS } from "@/utils/constants/actionNames";

export const apiKeysModule = {
    state: {
        apiKeys: [],
    },
    mutations: {
        [API_KEYS_MUTATIONS.FETCH](state: any, apiKeys: any[]) {
            state.apiKeys = apiKeys;
        },
        [API_KEYS_MUTATIONS.ADD](state: any, apiKey: any) {
            state.apiKeys.push(apiKey);
        },
        [API_KEYS_MUTATIONS.DELETE](state: any, id: string) {
            state.apiKeys = state.apiKeys.filter((key => key.id !== id))
        },
        [API_KEYS_MUTATIONS.TOGGLE_SELECTION](state: any, apiKeyID: string) {
            state.apiKeys = state.apiKeys.map((apiKey: any) => {
                if (apiKey.id === apiKeyID) {
                    apiKey.isSelected = !apiKey.isSelected;
                }

                return apiKey;
            });
        },
        [API_KEYS_MUTATIONS.CLEAR_SELECTION](state: any) {
            state.apiKeys = state.apiKeys.map((apiKey: any) => {
                apiKey.isSelected = false;

                return apiKey;
            });
        },
    },
    actions: {
        [API_KEYS_ACTIONS.FETCH]: async function ({commit, rootGetters}): Promise<RequestResponse<any>> {
            const projectId = rootGetters.selectedProject.id;

            let fetchResult = await fetchAPIKeys(projectId);

            if (fetchResult.isSuccess) {
                commit(API_KEYS_MUTATIONS.FETCH, fetchResult.data);
            }

            return fetchResult;
        },
        [API_KEYS_ACTIONS.CREATE]: async function ({commit, rootGetters}: any, name: string): Promise<RequestResponse<any>> {
            const projectId = rootGetters.selectedProject.id;

            let result = await createAPIKey(projectId, name);
            console.log(result);

            if (result.isSuccess) {
                commit(API_KEYS_MUTATIONS.ADD, result.data.keyInfo);
            }

            return result;
        },
        [API_KEYS_ACTIONS.DELETE]: async function({commit}: any, id: string): Promise<RequestResponse<any>> {
            let result = await deleteAPIKey(id);

            if (result.isSuccess) {
                commit(API_KEYS_MUTATIONS.DELETE, result.data.id);
            }

            console.log(result);

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
        selectedAPIKeys: function (state: any): any[] {
            let keys: any[] = state.apiKeys;
            let selectedKeys: any[] = [];

            for (let i = 0; i < keys.length; i++ ) {
                if (keys[i].isSelected) {
                    selectedKeys.push(keys[i]);
                }
            }

            return selectedKeys;
        }
    },
};
