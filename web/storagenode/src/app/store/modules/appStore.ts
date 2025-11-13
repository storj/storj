// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

class AppState {
    public constructor (
        public isSatelliteSelectionShown = false,
        public isBandwidthChartShown = true,
        public isEgressChartShown = false,
        public isIngressChartShown = false,
        public isPayoutCalendarShown = false,
        public isDarkMode = false,
        public isNoPayoutData = false,
        public isLoading = true,
        public isPayoutHistoryCalendarShown = false,
    ) {}
}

export const useAppStore = defineStore('appStore', () => {
    const state = reactive<AppState>(new AppState());

    function toggleSatelliteSelection(): void {
        state.isSatelliteSelectionShown = !state.isSatelliteSelectionShown;
    }

    function toggleBandwidthChart(): void {
        state.isBandwidthChartShown = !state.isBandwidthChartShown;
    }

    function toggleEgressChart(): void {
        if (!state.isBandwidthChartShown) {
            state.isEgressChartShown = !state.isEgressChartShown;
            state.isIngressChartShown = !state.isIngressChartShown;

            return;
        }

        state.isBandwidthChartShown = !state.isBandwidthChartShown;
        state.isEgressChartShown = !state.isEgressChartShown;
    }

    function toggleIngressChart(): void {
        if (!state.isBandwidthChartShown) {
            state.isIngressChartShown = !state.isIngressChartShown;
            state.isEgressChartShown = !state.isEgressChartShown;

            return;
        }

        state.isBandwidthChartShown = !state.isBandwidthChartShown;
        state.isIngressChartShown = !state.isIngressChartShown;
    }

    function togglePayoutCalendar(value: boolean): void {
        state.isPayoutCalendarShown = value;
    }

    function closeAdditionalCharts(): void {
        state.isBandwidthChartShown = true;
        state.isIngressChartShown = false;
        state.isEgressChartShown = false;
    }

    function setDarkMode(value: boolean): void {
        state.isDarkMode = value;
    }

    function setNoPayoutData(value: boolean): void {
        state.isNoPayoutData = value;
    }

    function setLoading(value: boolean): void {
        if (value) {
            state.isLoading = value;
        } else {
            setTimeout(() => { state.isLoading = value; }, 1000);
        }
    }

    function togglePayoutHistoryCalendar(value: boolean): void {
        state.isPayoutHistoryCalendarShown = value;
    }

    function closeAllPopups(): void {
        state.isSatelliteSelectionShown = false;
    }

    return {
        state,
        toggleSatelliteSelection,
        toggleBandwidthChart,
        toggleEgressChart,
        toggleIngressChart,
        togglePayoutCalendar,
        closeAdditionalCharts,
        setDarkMode,
        setNoPayoutData,
        setLoading,
        togglePayoutHistoryCalendar,
        closeAllPopups,
    };
});
