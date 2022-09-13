// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/types/store';
import { MetaUtils } from '@/utils/meta';
import { ABHitAction, ABTestApi, ABTestValues } from '@/types/abtesting';

export const AB_TESTING_ACTIONS = {
    FETCH: 'fetchAbTestValues',
    RESET: 'resetAbTestValues',
    HIT: 'sendHitEvent',
};

export const AB_TESTING_MUTATIONS = {
    SET: 'SET_AB_VALUES',
    INIT: 'SET_AB_INITIALIZED',
};

export class ABTestingState {
    public abTestValues = new ABTestValues();
    public abTestingEnabled = MetaUtils.getMetaContent('ab-testing-enabled') === 'true';
    public abTestingInitialized = false;
}

interface ABTestingContext {
    state: ABTestingState
    commit: (string, ...unknown) => void
    dispatch: (string, ...unknown) => Promise<any> // eslint-disable-line @typescript-eslint/no-explicit-any
}

const {
    FETCH,
    RESET,
    HIT,
} = AB_TESTING_ACTIONS;

const {
    SET,
    INIT,
} = AB_TESTING_MUTATIONS;

export function makeABTestingModule(api: ABTestApi): StoreModule<ABTestingState, ABTestingContext> {
    return {
        state: new ABTestingState(),
        mutations: {
            [SET](state: ABTestingState, values: ABTestValues): void {
                state.abTestValues = values;
            },
            [INIT](state: ABTestingState, isInitialized = true): void {
                state.abTestingInitialized = isInitialized;
            },
        },
        actions: {
            [FETCH]: async function ({ state, commit }: ABTestingContext): Promise<ABTestValues> {
                if (!state.abTestingEnabled)
                    return state.abTestValues;
                const values = await api.fetchABTestValues();

                await commit(SET, values);
                await commit(INIT);

                return values;
            },
            [RESET]: async function ({ commit }: ABTestingContext) {
                await commit(SET, new ABTestValues());
                await commit(INIT, false);
            },
            [HIT]: async function ({ state, dispatch }: ABTestingContext, action: ABHitAction) {
                if (!state.abTestingEnabled) return;
                if (!state.abTestingInitialized) {
                    await dispatch(FETCH);
                }
                switch (action) {
                case ABHitAction.UPGRADE_ACCOUNT_CLICKED:
                    api.sendHit(ABHitAction.UPGRADE_ACCOUNT_CLICKED).catch(_ => {});
                    break;
                }
            },
        },
    };
}
