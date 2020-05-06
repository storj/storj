// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import VerifiedStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/VerifiedStep.vue';

import { shallowMount } from '@vue/test-utils';

describe('VerifiedStep.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(VerifiedStep);

        expect(wrapper).toMatchSnapshot();
    });
});
