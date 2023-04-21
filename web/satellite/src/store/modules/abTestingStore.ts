// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { ABHitAction, ABTestApi, ABTestValues } from '@/types/abtesting';
import { ABHttpApi } from '@/api/abtesting';
import { useAppStore } from '@/store/modules/appStore';

export class ABTestingState {
    public abTestValues = new ABTestValues();
    public abTestingInitialized = false;
}

export const useABTestingStore = defineStore('abTesting', () => {
    const state = reactive<ABTestingState>(new ABTestingState());

    const api: ABTestApi = new ABHttpApi();

    const appStore = useAppStore();

    async function fetchValues(): Promise<ABTestValues> {
        if (!appStore.state.config.abTestingEnabled) return state.abTestValues;

        const values = await api.fetchABTestValues();

        state.abTestValues = values;
        state.abTestingInitialized = true;

        return values;
    }

    async function hit(action: ABHitAction): Promise<void> {
        if (!appStore.state.config.abTestingEnabled) return;
        if (!state.abTestingInitialized) {
            await fetchValues();
        }
        switch (action) {
        case ABHitAction.UPGRADE_ACCOUNT_CLICKED:
            api.sendHit(ABHitAction.UPGRADE_ACCOUNT_CLICKED).catch(_ => {});
            break;
        }
    }

    function reset(): void {
        state.abTestValues = new ABTestValues();
        state.abTestingInitialized = false;
    }

    return {
        fetchValues,
        hit,
        reset,
    };
});
