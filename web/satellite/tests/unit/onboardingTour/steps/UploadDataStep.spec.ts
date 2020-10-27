// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import UploadDataStep from '@/components/onboardingTour/steps/UploadDataStep.vue';

import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();

describe('UploadDataStep.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(UploadDataStep, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
