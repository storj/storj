// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { Currency } from '@/app/utils/currency';

import DetailsArea from '@/app/components/payouts/DetailsArea.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => Currency.dollarsFromCents(cents));

describe('DetailsArea', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(DetailsArea, {
            localVue,
            propsData: {
                totalEarned: 5000,
                totalPaid: 60000,
                totalHeld: 700,
                period: 'April, 2021',
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
