// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';

import { router } from '@/router';

import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';

const localVue = createLocalVue();

describe('OnboardingTourArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OnboardingTourArea, {
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
