// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const APPSTATE_MUTATIONS = {
    TOGGLE_SATELLITE_SELECTION: 'TOGGLE_SATELLITE_SELECTION',
    CLOSE_ALL: 'CLOSE_ALL',
};

export const APPSTATE_ACTIONS = {
    TOGGLE_SATELLITE_SELECTION: 'TOGGLE_SATELLITE_SELECTION',
};

const {
    TOGGLE_SATELLITE_SELECTION,
    CLOSE_ALL,
} = APPSTATE_MUTATIONS;

export const appStateModule = {
    state: {
        isSatelliteSelectionShown: false,
    },
    mutations: {
        [TOGGLE_SATELLITE_SELECTION](state: any): void {
            state.isSatelliteSelectionShown = !state.isSatelliteSelectionShown;
        },
        [CLOSE_ALL](state: any): void {
            state.isSatelliteSelectionShown = false;
        },
    },
    actions: {
        [APPSTATE_ACTIONS.TOGGLE_SATELLITE_SELECTION]: function ({commit, state}: any): void {
            if (!state.isSatelliteSelectionShown) {
                commit(APPSTATE_MUTATIONS.TOGGLE_SATELLITE_SELECTION);

                return;
            }

            commit(APPSTATE_MUTATIONS.CLOSE_ALL);
        },
    },
};
