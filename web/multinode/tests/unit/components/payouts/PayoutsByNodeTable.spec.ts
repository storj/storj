// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { Currency } from '@/app/utils/currency';
import { Paystub } from '@/payouts';
import { Size } from '@/private/memory/size';

import PayoutsByNodeTable from '@/app/components/payouts/tables/payoutsByNode/PayoutsByNodeTable.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => Currency.dollarsFromCents(cents));

localVue.filter('bytesToBase10String', (amountInBytes: number): string => Size.toBase10String(amountInBytes));

describe('PayoutsByNodeTable', (): void => {
    it('renders correctly', (): void => {
        const paystub = new Paystub(3000, 4000, 5000, 6000,
            50000, 60000, 70000, 80000, 90000, 100000, 110000);

        const wrapper = shallowMount(PayoutsByNodeTable, {
            localVue,
            propsData: { paystub },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
