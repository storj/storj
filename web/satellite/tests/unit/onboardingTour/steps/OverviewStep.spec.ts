// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import OverviewStep from '@/components/onboardingTour/steps/OverviewStep.vue';

import { mount } from '@vue/test-utils';

describe('OverviewStep.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = mount(OverviewStep);

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.get-started-button').trigger('click');

        expect(wrapper.emitted()).toHaveProperty('setAddPaymentState');
    });
});
