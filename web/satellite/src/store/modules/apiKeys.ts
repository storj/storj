// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { ApiKey, ApiKeyCursor, ApiKeyOrderBy, ApiKeysApi, ApiKeysPage } from '@/types/apiKeys';
import { SortDirection } from '@/types/common';

export const API_KEYS_MUTATIONS = {
    SET_PAGE: 'setApiKeys',
    TOGGLE_SELECTION: 'toggleApiKeysSelection',
    CLEAR_SELECTION: 'clearApiKeysSelection',
    CLEAR: 'clearApiKeys',
    CHANGE_SORT_ORDER: 'changeApiKeysSortOrder',
    CHANGE_SORT_ORDER_DIRECTION: 'changeApiKeysSortOrderDirection',
    SET_SEARCH_QUERY: 'setApiKeysSearchQuery',
    SET_PAGE_NUMBER: 'setApiKeysPage',
};

const {
    SET_PAGE,
    TOGGLE_SELECTION,
    CLEAR_SELECTION,
    CLEAR,
    CHANGE_SORT_ORDER,
    CHANGE_SORT_ORDER_DIRECTION,
    SET_SEARCH_QUERY,
    SET_PAGE_NUMBER,
} = API_KEYS_MUTATIONS;

export class ApiKeysState {
    public cursor: ApiKeyCursor = new ApiKeyCursor();
    public page: ApiKeysPage = new ApiKeysPage();
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
            [SET_PAGE](state: ApiKeysState, page: ApiKeysPage) {
                state.page = page;
            },
            [SET_PAGE_NUMBER](state: ApiKeysState, pageNumber: number) {
                state.cursor.page = pageNumber;
            },
            [SET_SEARCH_QUERY](state: ApiKeysState, search: string) {
                state.cursor.search = search;
            },
            [CHANGE_SORT_ORDER](state: ApiKeysState, order: ApiKeyOrderBy) {
                state.cursor.order = order;
            },
            [CHANGE_SORT_ORDER_DIRECTION](state: ApiKeysState, direction: SortDirection) {
                state.cursor.orderDirection = direction;
            },
            [TOGGLE_SELECTION](state: ApiKeysState, apiKeyID: string) {
                state.page.apiKeys = state.page.apiKeys.map((apiKey: ApiKey) => {
                    if (apiKey.id === apiKeyID) {
                        apiKey.isSelected = !apiKey.isSelected;
                    }

                    return apiKey;
                });
            },
            [CLEAR_SELECTION](state: ApiKeysState) {
                state.page.apiKeys = state.page.apiKeys.map((apiKey: ApiKey) => {
                    apiKey.isSelected = false;

                    return apiKey;
                });
            },
            [CLEAR](state: ApiKeysState) {
                state.cursor = new ApiKeyCursor();
                state.page = new ApiKeysPage();
            },
        },
        actions: {
            fetchApiKeys: async function ({commit, rootGetters, state}, pageNumber: number): Promise<ApiKeysPage> {
                const projectId = rootGetters.selectedProject.id;
                commit(SET_PAGE_NUMBER, pageNumber);

                const apiKeys = await api.get(projectId, state.cursor);
                commit(SET_PAGE, apiKeys);

                return apiKeys;
            },
            createApiKey: async function ({commit, rootGetters}: any, name: string): Promise<ApiKey> {
                const projectId = rootGetters.selectedProject.id;

                const apiKey = await api.create(projectId, name);

                return apiKey;
            },
            deleteApiKey: async function({commit}: any, ids: string[]): Promise<void> {
                return await api.delete(ids);
            },
            setApiKeysSearchQuery: function ({commit}, search: string) {
                commit(SET_SEARCH_QUERY, search);
            },
            setApiKeysSortingBy: function ({commit}, order: ApiKeyOrderBy) {
                commit(CHANGE_SORT_ORDER, order);
            },
            setApiKeysSortingDirection: function ({commit}, direction: SortDirection) {
                commit(CHANGE_SORT_ORDER_DIRECTION, direction);
            },
            toggleApiKeySelection: function ({commit}, apiKeyID: string): void {
                commit(TOGGLE_SELECTION, apiKeyID);
            },
            clearApiKeySelection: function ({commit}): void {
                commit(CLEAR_SELECTION);
            },
            clearApiKeys: function ({commit}): void {
                commit(CLEAR);
            },
        },
        getters: {
            selectedApiKeys: (state: ApiKeysState) => state.page.apiKeys.filter((key: ApiKey) => key.isSelected),
            apiKeys: function (state: ApiKeysState): ApiKey[] {
                return state.page.apiKeys;
            },
        },
    };
}
