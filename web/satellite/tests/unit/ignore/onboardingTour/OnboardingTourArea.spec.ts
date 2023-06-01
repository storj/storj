// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import OnboardingTourArea from '@/components/onboardingTour/OnboardingTourArea.vue';

describe('OnboardingTourArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OnboardingTourArea);

        expect(wrapper).toMatchSnapshot();
    });
});
