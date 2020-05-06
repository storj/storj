// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';

import UploadDataStep from '@/components/onboardingTour/steps/UploadDataStep.vue';

import { router } from '@/router';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();

describe('UploadDataStep.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(UploadDataStep, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('button click works correctly', async (): Promise<void> => {
        const clickSpy = sinon.spy();
        const wrapper = mount(UploadDataStep, {
            localVue,
            router,
            methods: {
                onButtonClick: clickSpy,
            },
        });

        await wrapper.find('.go-to-button').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
