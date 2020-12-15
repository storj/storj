// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import SaveApiKeyModal from '@/components/onboardingTour/steps/SaveApiKeyModal.vue';

import { mount } from '@vue/test-utils';

describe('SaveApiKeyModal.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(SaveApiKeyModal);

        expect(wrapper).toMatchSnapshot();
    });
});
