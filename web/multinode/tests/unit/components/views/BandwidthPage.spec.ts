// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import store from '../../mock/store';

import BandwidthPage from '@/app/views/bandwidth/BandwidthPage.vue';
import { BandwidthTraffic } from '@/bandwidth';
import { Size } from '@/private/memory/size';

const localVue = createLocalVue();

localVue.use(Vuex);

localVue.filter('bytesToBase10String', (amountInBytes: number): string => Size.toBase10String(amountInBytes));

const traffic = new BandwidthTraffic();

traffic.bandwidthSummary = 700000000;
traffic.egressSummary = 577700000000;
traffic.ingressSummary = 5000000;

describe('BandwidthPage', (): void => {
    it('renders correctly', (): void => {
        store.commit('bandwidth/populate', traffic);

        const wrapper = shallowMount(BandwidthPage, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with egress chart', async(): Promise<void> => {
        const wrapper = shallowMount(BandwidthPage, {
            store,
            localVue,
        });

        await wrapper.findAll('.chart-container__title-area__chart-choice-item').at(1).trigger('click');

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with ingress chart', async(): Promise<void> => {
        const wrapper = shallowMount(BandwidthPage, {
            store,
            localVue,
        });

        await wrapper.findAll('.chart-container__title-area__chart-choice-item').at(2).trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
