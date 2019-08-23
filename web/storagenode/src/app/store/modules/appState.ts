// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { APPSTATE_ACTIONS, APPSTATE_MUTATIONS } from '@/app/utils/constants';

export const appStateModule = {
    state: {
        isSatelliteSelectionShown: false,
    },
    mutations: {
        [APPSTATE_MUTATIONS.TOGGLE_SATELLITE_SELECTION](state: any): void {
            state.isSatelliteSelectionShown = !state.isSatelliteSelectionShown;
        },
        [APPSTATE_MUTATIONS.CLOSE_ALL](state: any): void {
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
