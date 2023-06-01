// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';

// TODO: figure out how to fix the test
xdescribe('OverviewStep.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OverviewStep);

        expect(wrapper).toMatchSnapshot();
    });
});
