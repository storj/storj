// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import AddCardState from '@/components/onboardingTour/steps/paymentStates/AddCardState.vue';

import { shallowMount } from '@vue/test-utils';

describe('AddCardState.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(AddCardState);

        expect(wrapper).toMatchSnapshot();
    });
});
