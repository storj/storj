// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { Currency } from '@/app/utils/currency';

import BalanceArea from '@/app/components/payouts/BalanceArea.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => Currency.dollarsFromCents(cents));

describe('BalanceArea', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(BalanceArea, {
            localVue,
            propsData: {
                currentMonthEstimation: 66000,
                undistributed: 1000,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
