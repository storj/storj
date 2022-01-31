// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const APPSTATE_MUTATIONS = {
    TOGGLE_SATELLITE_SELECTION: 'TOGGLE_SATELLITE_SELECTION',
    TOGGLE_BANDWIDTH_CHART: 'TOGGLE_BANDWIDTH_CHART',
    TOGGLE_EGRESS_CHART: 'TOGGLE_EGRESS_CHART',
    TOGGLE_INGRESS_CHART: 'TOGGLE_INGRESS_CHART,',
    TOGGLE_PAYOUT_CALENDAR: 'TOGGLE_PAYOUT_CALENDAR',
    CLOSE_ADDITIONAL_CHARTS: 'CLOSE_ADDITIONAL_CHARTS',
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
    TOGGLE_INGRESS_CHART: 'TOGGLE_INGRESS_CHART',
    CLOSE_ADDITIONAL_CHARTS: 'CLOSE_ADDITIONAL_CHARTS',
    TOGGLE_PAYOUT_CALENDAR: 'TOGGLE_PAYOUT_CALENDAR',
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
    TOGGLE_INGRESS_CHART,
    CLOSE_ADDITIONAL_CHARTS,
    CLOSE_ALL_POPUPS,
    TOGGLE_PAYOUT_CALENDAR,
} = APPSTATE_MUTATIONS;

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

interface AppContext {
    state: AppState;
    commit: (string, ...unknown) => void;
}

export const appStateModule = {
    state: new AppState(),
    mutations: {
        [TOGGLE_SATELLITE_SELECTION](state: AppState): void {
            state.isSatelliteSelectionShown = !state.isSatelliteSelectionShown;
        },
        [TOGGLE_BANDWIDTH_CHART](state: AppState): void {
            state.isBandwidthChartShown = !state.isBandwidthChartShown;
        },
        [TOGGLE_EGRESS_CHART](state: AppState): void {
            state.isEgressChartShown = !state.isEgressChartShown;
        },
        [TOGGLE_INGRESS_CHART](state: AppState): void {
            state.isIngressChartShown = !state.isIngressChartShown;
        },
        [TOGGLE_PAYOUT_CALENDAR](state: AppState, value: boolean): void {
            state.isPayoutCalendarShown = value;
        },
        [CLOSE_ADDITIONAL_CHARTS](state: AppState): void {
            state.isBandwidthChartShown = true;
            state.isIngressChartShown = false;
            state.isEgressChartShown = false;
        },
        [APPSTATE_MUTATIONS.SET_DARK](state: AppState, value: boolean): void {
            state.isDarkMode = value;
        },
        [APPSTATE_MUTATIONS.SET_NO_PAYOUT_INFO](state: AppState, value: boolean): void {
            state.isNoPayoutData = value;
        },
        [APPSTATE_MUTATIONS.SET_LOADING_STATE](state: AppState, value: boolean): void {
            state.isLoading = value;
        },
        [CLOSE_ALL_POPUPS](state: AppState): void {
            state.isSatelliteSelectionShown = false;
        },
        [APPSTATE_MUTATIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR](state: AppState, value: boolean): void {
            state.isPayoutHistoryCalendarShown = value;
        },
    },
    actions: {
        [APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION]: function ({commit, state}: AppContext): void {
            if (!state.isSatelliteSelectionShown) {
                commit(APPSTATE_MUTATIONS.TOGGLE_SATELLITE_SELECTION);

                return;
            }

            commit(APPSTATE_MUTATIONS.CLOSE_ALL_POPUPS);
        },
        [APPSTATE_ACTIONS.TOGGLE_PAYOUT_CALENDAR]: function ({commit}: AppContext, value: boolean): void {
            commit(APPSTATE_MUTATIONS.TOGGLE_PAYOUT_CALENDAR, value);
        },
        [APPSTATE_ACTIONS.TOGGLE_EGRESS_CHART]: function ({commit, state}: AppContext): void {
            if (!state.isBandwidthChartShown) {
                commit(APPSTATE_MUTATIONS.TOGGLE_EGRESS_CHART);
                commit(APPSTATE_MUTATIONS.TOGGLE_INGRESS_CHART);

                return;
            }

            commit(APPSTATE_MUTATIONS.TOGGLE_BANDWIDTH_CHART);
            commit(APPSTATE_MUTATIONS.TOGGLE_EGRESS_CHART);
        },
        [APPSTATE_ACTIONS.TOGGLE_INGRESS_CHART]: function ({commit, state}: AppContext): void {
            if (!state.isBandwidthChartShown) {
                commit(APPSTATE_MUTATIONS.TOGGLE_INGRESS_CHART);
                commit(APPSTATE_MUTATIONS.TOGGLE_EGRESS_CHART);

                return;
            }

            commit(APPSTATE_MUTATIONS.TOGGLE_BANDWIDTH_CHART);
            commit(APPSTATE_MUTATIONS.TOGGLE_INGRESS_CHART);
        },
        [APPSTATE_ACTIONS.SET_DARK_MODE]: function ({commit}: AppContext, value: boolean): void {
            commit(APPSTATE_MUTATIONS.SET_DARK, value);
        },
        [APPSTATE_ACTIONS.SET_NO_PAYOUT_DATA]: function ({commit}: AppContext, value: boolean): void {
            commit(APPSTATE_MUTATIONS.SET_NO_PAYOUT_INFO, value);
        },
        [APPSTATE_ACTIONS.SET_LOADING]: function ({commit}: AppContext, value: boolean): void {
            value ? commit(APPSTATE_MUTATIONS.SET_LOADING_STATE, value) :
                setTimeout(() => { commit(APPSTATE_MUTATIONS.SET_LOADING_STATE, value); }, 1000);
        },
        [APPSTATE_ACTIONS.CLOSE_ADDITIONAL_CHARTS]: function ({commit}: AppContext): void {
            commit(APPSTATE_MUTATIONS.CLOSE_ADDITIONAL_CHARTS);
        },
        [APPSTATE_ACTIONS.CLOSE_ALL_POPUPS]: function ({commit}: AppContext): void {
            commit(APPSTATE_MUTATIONS.CLOSE_ALL_POPUPS);
        },
        [APPSTATE_ACTIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR]: function ({commit}: AppContext, value: boolean): void {
            commit(APPSTATE_MUTATIONS.TOGGLE_PAYOUT_HISTORY_CALENDAR, value);
        },
    },
};
