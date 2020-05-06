// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import VerifyingStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/VerifyingStep.vue';

import { shallowMount } from '@vue/test-utils';

describe('VerifyingStep.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(VerifyingStep);

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.verifying-step__back-button').trigger('click');

        expect(wrapper.emitted()).toEqual({ 'setDefaultState': [[]] });
    });
});
