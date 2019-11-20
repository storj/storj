// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const APPSTATE_MUTATIONS = {
    TOGGLE_SATELLITE_SELECTION: 'TOGGLE_SATELLITE_SELECTION',
    TOGGLE_BANDWIDTH_CHART: 'TOGGLE_BANDWIDTH_CHART',
    TOGGLE_EGRESS_CHART: 'TOGGLE_EGRESS_CHART',
    CLOSE_ALL_POPUPS: 'CLOSE_ALL_POPUPS',
    SET_DARK: 'SET_DARK',
    SET_NO_PAYOUT_INFO: 'SET_NO_PAYOUT_INFO',
    SET_LOADING_STATE: 'SET_LOADING_STATE',
    TOGGLE_PAYOUT_HISTORY_CALENDAR: 'TOGGLE_PAYOUT_HISTORY_CALENDAR',
};

export const APPSTATE_ACTIONS = {
    TOGGLE_SATELLITE_SELECTION: 'TOGGLE_SATELLITE_SELECTION',
    TOGGLE_BANDWIDTH_CHART: 'TOGGLE_BANDWIDTH_CHART',
    TOGGLE_EGRESS_CHART: 'TOGGLE_EGRESS_CHART',
    CLOSE_ALL_POPUPS: 'CLOSE_ALL_POPUPS',
    SET_DARK_MODE: 'SET_DARK_MODE',
    SET_NO_PAYOUT_DATA: 'SET_NO_PAYOUT_DATA',
    SET_LOADING: 'SET_LOADING',
    TOGGLE_PAYOUT_HISTORY_CALENDAR: 'TOGGLE_PAYOUT_HISTORY_CALENDAR',
};

const {
    TOGGLE_SATELLITE_SELECTION,
    TOGGLE_BANDWIDTH_CHART,
    TOGGLE_EGRESS_CHART,
    CLOSE_ALL_POPUPS,
    TOGGLE_PAYOUT_CALENDAR,
} = APPSTATE_MUTATIONS;

export const appStateModule = {
    state: {
        isSatelliteSelectionShown: false,
        isBandwidthChartShown: true,
        isEgressChartShown: false,
    },
    mutations: {
        [TOGGLE_SATELLITE_SELECTION](state: any): void {
            state.isSatelliteSelectionShown = !state.isSatelliteSelectionShown;
        },
        [TOGGLE_BANDWIDTH_CHART](state: any): void {
            state.isBandwidthChartShown = !state.isBandwidthChartShown;
        },
        [TOGGLE_EGRESS_CHART](state: any): void {
            state.isEgressChartShown = !state.isEgressChartShown;
        },
        [CLOSE_ALL_POPUPS](state: any): void {
            state.isSatelliteSelectionShown = false;
        },
        [APPSTATE_MUTATIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR](state: any, value): void {
            state.isPayoutHistoryCalendarShown = value;
        },
    },
    actions: {
        [APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION]: function ({commit, state}: any): void {
            if (!state.isSatelliteSelectionShown) {
                commit(APPSTATE_MUTATIONS.TOGGLE_SATELLITE_SELECTION);

                return;
            }

            commit(APPSTATE_MUTATIONS.CLOSE_ALL_POPUPS);
        },
        [APPSTATE_ACTIONS.TOGGLE_EGRESS_CHART]: function ({commit}: any): void {
            commit(APPSTATE_MUTATIONS.TOGGLE_BANDWIDTH_CHART);
            commit(APPSTATE_MUTATIONS.TOGGLE_EGRESS_CHART);
        },
        [APPSTATE_ACTIONS.CLOSE_ALL_POPUPS]: function ({commit}: any): void {
            commit(APPSTATE_MUTATIONS.CLOSE_ALL_POPUPS);
        },
        [APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR]: function ({commit, state}: any, value: boolean): void {
            commit(APPSTATE_MUTATIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, value);
        },
    },
};
