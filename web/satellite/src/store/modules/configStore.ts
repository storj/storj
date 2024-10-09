// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, ComputedRef, reactive } from 'vue';
import { defineStore } from 'pinia';

import { FrontendConfig, FrontendConfigApi } from '@/types/config';
import { FrontendConfigHttpApi } from '@/api/config';
import { NavigationLink } from '@/types/navigation';
import { RouteConfig } from '@/types/router';

export class ConfigState {
    public config: FrontendConfig = new FrontendConfig();
}

export const useConfigStore = defineStore('config', () => {
    const state = reactive<ConfigState>(new ConfigState());

    const configApi: FrontendConfigApi = new FrontendConfigHttpApi();

    const firstOnboardingStep = computed((): NavigationLink => {
        return state.config.pricingPackagesEnabled ? RouteConfig.PricingPlanStep : RouteConfig.OverviewStep;
    });

    /**
     * This is whether the UI for object locking is globally enabled or not.
     * It is a combination of whether the object lock feature itself is enabled
     * in metainfo and another flag of the same name console.
     */
    const objectLockUIEnabled: ComputedRef<boolean> = computed(() => state.config.objectLockUIEnabled);

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
        objectLockUIEnabled,
        getConfig,
        getBillingEnabled,
    };
});
