// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import { FrontendConfig, FrontendConfigApi } from '@/types/config';
import { FrontendConfigHttpApi } from '@/api/config';
import { NavigationLink } from '@/types/navigation';
import { RouteConfig } from '@/types/router';
import { User } from '@/types/users';

export class ConfigState {
    public config: FrontendConfig = new FrontendConfig();
}

export const useConfigStore = defineStore('config', () => {
    const state = reactive<ConfigState>(new ConfigState());

    const configApi: FrontendConfigApi = new FrontendConfigHttpApi();

    const firstOnboardingStep = computed((): NavigationLink => {
        return state.config.pricingPackagesEnabled ? RouteConfig.PricingPlanStep : RouteConfig.OverviewStep;
    });

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
        firstOnboardingStep,
        getConfig,
        getBillingEnabled,
    };
});
