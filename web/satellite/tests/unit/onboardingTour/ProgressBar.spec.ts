// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import ProgressBar from '@/components/onboardingTour/ProgressBar.vue';

import { mount } from '@vue/test-utils';

describe('ProgressBar.vue', () => {
    it('renders correctly if paywall is enabled', (): void => {
        const wrapper = mount(ProgressBar, {
            propsData: {
                isPaywallEnabled: true,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if paywall is disabled', (): void => {
        const wrapper = mount(ProgressBar, {
            propsData: {
                isPaywallEnabled: false,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if add payment step is completed', (): void => {
        const wrapper = mount(ProgressBar, {
            propsData: {
                isAddPaymentStep: true,
                isPaywallEnabled: true,
            },
        });

        expect(wrapper.findAll('.completed-step').length).toBe(1);
        expect(wrapper.findAll('.completed-font-color').length).toBe(1);
    });

    it('renders correctly if create project step is completed', (): void => {
        const wrapper = mount(ProgressBar, {
            propsData: {
                isCreateProjectStep: true,
                isPaywallEnabled: true,
            },
        });

        expect(wrapper.findAll('.completed-step').length).toBe(3);
        expect(wrapper.findAll('.completed-font-color').length).toBe(2);
    });

    it('renders correctly if create api key step is completed', (): void => {
        const wrapper = mount(ProgressBar, {
            propsData: {
                isCreateApiKeyStep: true,
                isPaywallEnabled: true,
            },
        });

        expect(wrapper.findAll('.completed-step').length).toBe(5);
        expect(wrapper.findAll('.completed-font-color').length).toBe(3);
    });

    it('renders correctly if upload data step is completed', (): void => {
        const wrapper = mount(ProgressBar, {
            propsData: {
                isUploadDataStep: true,
                isPaywallEnabled: true,
            },
        });

        expect(wrapper.findAll('.completed-step').length).toBe(7);
        expect(wrapper.findAll('.completed-font-color').length).toBe(4);
    });
});
