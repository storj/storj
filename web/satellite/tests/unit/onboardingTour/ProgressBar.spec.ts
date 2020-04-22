// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import ProgressBar from '@/components/onboardingTour/ProgressBar.vue';

import { mount } from '@vue/test-utils';

describe('ProgressBar.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(ProgressBar);

        expect(wrapper).toMatchSnapshot();
    });
});
