// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive } from 'vue';
import { defineStore } from 'pinia';

import { FrontendConfig, FrontendConfigApi } from '@/types/config';
import { FrontendConfigHttpApi } from '@/api/config';
import { centsToDollars } from '@/utils/strings';
import { Time } from '@/utils/time';

export class ConfigState {
    public config: FrontendConfig = new FrontendConfig();
}

export const useConfigStore = defineStore('config', () => {
    const state = reactive<ConfigState>(new ConfigState());

    const configApi: FrontendConfigApi = new FrontendConfigHttpApi();

    const minimumCharge = computed<MinimumCharge>(() => {
        if (!state.config.minimumCharge) {
            return new MinimumCharge();
        }
        return new MinimumCharge(
            state.config.minimumCharge.enabled,
            state.config.minimumCharge.amount,
            state.config.minimumCharge.startDate,
        );
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
        minimumCharge,
        getConfig,
        getBillingEnabled,
    };
});

/**
 * MinimumCharge represents minimum charge config.
 */
export class MinimumCharge {
    public constructor(
        public enabled = false,
        public _amount = 0,
        public _startDate: string | null = null,
    ) { }

    get amount(): string {
        return centsToDollars(this._amount);
    }

    get startDate(): Date | null {
        return this._startDate !== null ? new Date(this._startDate) : null;
    }

    get shortStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        return Time.formattedDate(this.startDate, { month: 'long', day: 'numeric' });
    }

    get longStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        return Time.formattedDate(this.startDate, { month: 'long', day: 'numeric', year: 'numeric' });
    }

    get proNoticeEnabled(): boolean {
        return this.enabled && this.startDate !== null && this.startDate > new Date();
    }
}
