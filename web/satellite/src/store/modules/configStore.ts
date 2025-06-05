// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { computed, reactive, ref } from 'vue';
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

    const minimumChargeBannerDismissed = ref(false);

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
        minimumChargeBannerDismissed,
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

    get monthYearStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        return Time.formattedDate(this.startDate, { month: 'long', year: 'numeric', timeZone: 'UTC' });
    }

    get shortStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        const str = Time.formattedDate(this.startDate, { month: 'long', day: 'numeric', timeZone: 'UTC', timeZoneName: 'short' });
        const parts = str.split(' at');
        return parts.join('');
    }

    get longStartDateStr(): string {
        if (!this.startDate) {
            return '';
        }
        const str = Time.formattedDate(this.startDate, { month: 'long', day: 'numeric', year: 'numeric', timeZone: 'UTC', timeZoneName: 'short' });
        const parts = str.split(' at');
        return parts.join('');
    }

    // notice is enabled 60 days before the start date and 45 days after the start date.
    get priorNoticeEnabled(): boolean {
        if (!this.enabled || !this.startDate) return false;

        const currentDate = new Date();
        const startDate = this.startDate;
        const sixtyDaysBefore = new Date(startDate);
        sixtyDaysBefore.setDate(sixtyDaysBefore.getDate() - 60);
        const forty5DaysAfter = new Date(startDate);
        forty5DaysAfter.setDate(forty5DaysAfter.getDate() + 45);

        return currentDate >= sixtyDaysBefore && currentDate <= forty5DaysAfter;
    }

    // notice is enabled after 45 days from the start date.
    get noticeEnabled(): boolean {
        if (!this.enabled || !this.startDate) return false;

        const currentDate = new Date();
        const startDate = this.startDate;
        const forty5DaysAfter = new Date(startDate);
        forty5DaysAfter.setDate(forty5DaysAfter.getDate() + 45);

        return currentDate > forty5DaysAfter;
    }
}
