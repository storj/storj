// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { CreditsApiGql } from '@/api/credits';
import { CREDIT_USAGE_ACTIONS, CREDIT_USAGE_MUTATIONS, makeCreditsModule } from '@/store/modules/credits';
import { CreditUsage } from '@/types/credits';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();
const api = new CreditsApiGql();
const creditsModule = makeCreditsModule(api);
const { FETCH } = CREDIT_USAGE_ACTIONS;
const { SET, CLEAR } = CREDIT_USAGE_MUTATIONS;

Vue.use(Vuex);

const store = new Vuex.Store(creditsModule);

describe('mutations', () => {
    it('set credits', () => {
        const credits = new CreditUsage(1, 2, 3);

        store.commit(SET, credits);

        expect(store.state.usedCredits).toBe(credits.usedCredits);
        expect(store.state.referred).toBe(credits.referred);
        expect(store.state.availableCredits).toBe(credits.availableCredits);
    });
    it('clear credits', () => {
        store.commit(CLEAR);

        expect(store.state.usedCredits).toBe(0);
        expect(store.state.referred).toBe(0);
        expect(store.state.availableCredits).toBe(0);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });
    it('successfully get credits', async () => {
        const credits = new CreditUsage(3, 4, 5);

        jest.spyOn(api, 'get').mockReturnValue(
            Promise.resolve(credits),
        );

        await store.dispatch(FETCH);

        expect(store.state.usedCredits).toBe(credits.usedCredits);
        expect(store.state.referred).toBe(credits.referred);
        expect(store.state.availableCredits).toBe(credits.availableCredits);
    });

    it('clear action dispatched successfully', async () => {
        await store.dispatch(CREDIT_USAGE_ACTIONS.CLEAR);

        expect(store.state.usedCredits).toBe(0);
        expect(store.state.referred).toBe(0);
        expect(store.state.availableCredits).toBe(0);

    });

    it('get throws an error when api call fails', async () => {
        const credits = new CreditUsage(3, 4, 5);

        jest.spyOn(api, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(FETCH);
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.usedCredits).toBe(0);
            expect(store.state.referred).toBe(0);
            expect(store.state.availableCredits).toBe(0);
        }
    });
});

describe('getters', () => {
    it('credits', function () {
        const credits = store.getters.credits;

        expect(credits.availableCredits).toBe(store.state.availableCredits);
        expect(credits.referred).toBe(store.state.referred);
        expect(credits.usedCredits).toBe(store.state.usedCredits);
    });
});
