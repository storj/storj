// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import {
    PayoutInfoRange,
    PayoutState,
} from '@/app/types/payout';
import { getHeldPercentage } from '@/app/utils/payout';
import { PayoutPeriod } from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { useNodeStore } from '@/app/store/modules/nodeStore';
import { SatelliteInfo } from '@/storagenode/sno/sno';

export const usePayoutStore = defineStore('payoutStore', () => {
    const state = reactive<PayoutState>(new PayoutState());

    const service: PayoutService = new PayoutService(new PayoutHttpApi());

    const nodeStore = useNodeStore();
    const selectedSatellite = computed<SatelliteInfo>(() => nodeStore.state.selectedSatellite);

    const totalPaidForPayoutHistoryPeriod = computed<number>(() => {
        return state.payoutHistory.map(data => data.paid).reduce((previous, current) => previous + current, 0);
    });

    async function fetchPayoutInfo(satelliteId = ''): Promise<void> {
        const totalPaystubForPeriod = await service.paystubSummaryForPeriod(
            state.periodRange.start,
            state.periodRange.end,
            satelliteId,
        );

        state.heldPercentage = getHeldPercentage(selectedSatellite.value.joinDate);
        state.totalPaystubForPeriod = totalPaystubForPeriod;
    }

    async function fetchTotalPayments(satelliteId = ''): Promise<void> {
        const now = new Date();
        const start = new PayoutPeriod(selectedSatellite.value.joinDate.getUTCFullYear(), selectedSatellite.value.joinDate.getUTCMonth());
        const end = new PayoutPeriod(now.getUTCFullYear(), now.getUTCMonth());

        const totalPayments = await service.totalPayments(start, end, satelliteId);

        state.heldPercentage = getHeldPercentage(selectedSatellite.value.joinDate);
        state.totalPayments = totalPayments;
    }

    function setPeriodsRange(periodRange: PayoutInfoRange): void {
        state.periodRange = periodRange;
    }

    async function fetchHeldHistory(): Promise<void> {
        state.heldHistory = await service.allSatellitesHeldHistory();
    }

    async function getPeriods(satelliteId = ''): Promise<void> {
        const periods = await service.availablePeriods(satelliteId);

        state.payoutPeriods = periods;

        if (!satelliteId) {
            state.payoutHistoryAvailablePeriods = periods;
        }
    }

    async function fetchEstimation(satelliteId = ''): Promise<void> {
        const estimatedInfo = await service.estimatedPayout(satelliteId);

        state.estimation = estimatedInfo;
        state.currentMonthEarnings = estimatedInfo.currentMonth.payout + estimatedInfo.currentMonth.held;
    }

    async function fetchPricingModel(satelliteId: string): Promise<void> {
        state.pricingModel = await service.pricingModel(satelliteId);
    }

    async function fetchPayoutHistory(): Promise<void> {
        if (!state.payoutHistoryPeriod) return;

        state.payoutHistory = await service.payoutHistory(state.payoutHistoryPeriod);
    }

    function setPayoutHistoryPeriod(period: string): void {
        state.payoutHistoryPeriod = period;
    }

    return {
        state,
        totalPaidForPayoutHistoryPeriod,
        fetchTotalPayments,
        setPeriodsRange,
        fetchHeldHistory,
        getPeriods,
        fetchEstimation,
        fetchPricingModel,
        fetchPayoutHistory,
        setPayoutHistoryPeriod,
        fetchPayoutInfo,
    };
});
