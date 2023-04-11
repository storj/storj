// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { router } from '@/router';

import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

// TODO: figure out how to fix the test
xdescribe('OverviewStep.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OverviewStep, {
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
