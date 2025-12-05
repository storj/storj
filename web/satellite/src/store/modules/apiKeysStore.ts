// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import { useConfigStore } from '@/store/modules/configStore';
import { RestApiKey } from '@/types/restApiKeys';
import { ApiKeysHttpApi } from '@/api/apiKeys';

class RestApiKeysState {
    public keys: RestApiKey[] = [];
}

export const useRestApiKeysStore = defineStore('apiKeys', () => {
    const api = new ApiKeysHttpApi();

    const state = reactive<RestApiKeysState>(new RestApiKeysState());

    const configStore = useConfigStore();

    const csrfToken = computed<string>(() => configStore.state.config.csrfToken);

    async function getKeys(): Promise<RestApiKey[]> {
        const keys = await api.getAll();
        state.keys = keys;

        return keys;
    }

    async function createAPIKey(name: string, expiration: number): Promise<string> {
        return await api.create(name, expiration || null, csrfToken.value);
    }

    async function deleteAPIKeys(ids: string[]): Promise<void> {
        await api.delete(ids, csrfToken.value);
    }

    function clear(): void {
        state.keys = [];
    }

    return {
        state,
        getKeys,
        createAPIKey,
        deleteAPIKeys,
        clear,
    };
});
