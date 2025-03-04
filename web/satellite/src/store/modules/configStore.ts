// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { FrontendConfig, FrontendConfigApi } from '@/types/config';
import { FrontendConfigHttpApi } from '@/api/config';

export class ConfigState {
    public config: FrontendConfig = new FrontendConfig();
}

export const useConfigStore = defineStore('config', () => {
    const state = reactive<ConfigState>(new ConfigState());

    const configApi: FrontendConfigApi = new FrontendConfigHttpApi();

    async function getConfig(): Promise<FrontendConfig> {
        const result = await configApi.get();

        state.config = result;

        return result;
    }

    function getBillingEnabled(hasVarPartner: boolean): boolean {
        return state.config.billingFeaturesEnabled && !hasVarPartner;
    }

    return {
        state,
        getConfig,
        getBillingEnabled,
    };
});
