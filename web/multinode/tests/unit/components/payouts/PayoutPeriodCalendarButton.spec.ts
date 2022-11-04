// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import PayoutPeriodCalendarButton from '@/app/components/payouts/PayoutPeriodCalendarButton.vue';

const localVue = createLocalVue();

localVue.use(Vuex);

describe('PayoutPeriodCalendarButton', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PayoutPeriodCalendarButton, {
            localVue,
            propsData: {
                period: 'April, 2021',
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('triggers open calendar correctly', async(): Promise<void> => {
        const wrapper = shallowMount(PayoutPeriodCalendarButton, {
            localVue,
            propsData: {
                period: 'April, 2021',
            },
        });

        await wrapper.find('.calendar-button').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
