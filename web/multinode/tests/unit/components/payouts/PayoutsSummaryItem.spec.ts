// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import PayoutsSummaryItem from '@/app/components/payouts/tables/payoutSummary/PayoutsSummaryItem.vue';

import { Currency } from '@/app/utils/currency';
import { NodePayoutsSummary } from '@/payouts';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => {
    return Currency.dollarsFromCents(cents);
});

describe('PayoutsSummaryItem', (): void => {
    it('renders correctly', (): void => {
        const payoutsSummary = new NodePayoutsSummary('1', 'name1', 5000, 4000);

        const wrapper = shallowMount(PayoutsSummaryItem, {
            localVue,
            propsData: { payoutsSummary },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
