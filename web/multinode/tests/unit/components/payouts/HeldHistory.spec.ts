// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { Currency } from '@/app/utils/currency';
import { HeldAmountSummary } from '@/payouts';

import HeldHistory from '@/app/components/payouts/tables/heldHistory/HeldHistory.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => Currency.dollarsFromCents(cents));

describe('HeldHistory', (): void => {
    it('renders correctly', (): void => {
        const heldHistory = [
            new HeldAmountSummary('satelliteName', 100000, 200000, 300000, 10),
            new HeldAmountSummary('satelliteName', 200000, 300000, 400000, 20),
        ];

        const wrapper = shallowMount(HeldHistory, {
            localVue,
            propsData: { heldHistory },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
